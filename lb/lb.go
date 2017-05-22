package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
)

// TODO: consistent hashing is nice to get a cheap way to place nodes but it
// doesn't account well for certain functions that may be 'hotter' than others.
// we should very likely keep a load ordered list and distribute based on that.
// if we can get some kind of feedback from the f(x) nodes, we can use that.
// maybe it's good enough to just ch(x) + 1 if ch(x) is marked as "hot"?

// TODO the load balancers all need to have the same list of nodes. gossip?
// also gossip would handle failure detection instead of elb style. or it can
// be pluggable and then we can read from where bmc is storing them and use that
// or some OSS alternative

// TODO when adding nodes we should health check them once before adding them
// TODO when node goes offline should try to redirect request instead of 5xxing

// TODO we could add some kind of pre-warming call to the functions server where
// the lb could send an image to it to download before the lb starts sending traffic
// there, otherwise when load starts expanding a few functions are going to eat
// the pull time

// TODO config
// TODO TLS

func main() {
	// XXX (reed): normalize
	fnodes := flag.String("nodes", "", "comma separated list of IronFunction nodes")

	var conf config
	flag.IntVar(&conf.Port, "port", 8081, "port to run on")
	flag.IntVar(&conf.HealthcheckInterval, "hc-interval", 3, "how often to check f(x) nodes, in seconds")
	flag.StringVar(&conf.HealthcheckEndpoint, "hc-path", "/version", "endpoint to determine node health")
	flag.IntVar(&conf.HealthcheckUnhealthy, "hc-unhealthy", 2, "threshold of failed checks to declare node unhealthy")
	flag.IntVar(&conf.HealthcheckTimeout, "hc-timeout", 5, "timeout of healthcheck endpoint, in seconds")
	flag.Parse()

	conf.Nodes = strings.Split(*fnodes, ",")

	ch := newProxy(conf)

	// XXX (reed): safe shutdown
	fmt.Println(http.ListenAndServe(":8081", ch))
}

type config struct {
	Port                 int      `json:"port"`
	Nodes                []string `json:"nodes"`
	HealthcheckInterval  int      `json:"healthcheck_interval"`
	HealthcheckEndpoint  string   `json:"healthcheck_endpoint"`
	HealthcheckUnhealthy int      `json:"healthcheck_unhealthy"`
	HealthcheckTimeout   int      `json:"healthcheck_timeout"`
}

// XXX (reed): clean up mess
var dashPage []byte

func init() {
	jsb, err := ioutil.ReadFile("dash.js")
	if err != nil {
		logrus.WithError(err).Fatal("couldn't open dash.js file")
	}

	dashPage = []byte(fmt.Sprintf(dashStr, string(jsb)))
}

func (ch *chProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	// XXX (reed): probably do these on a separate port to avoid conflicts
	case "/1/lb/nodes":
		switch r.Method {
		case "PUT":
			ch.addNode(w, r)
			return
		case "DELETE":
			ch.removeNode(w, r)
			return
		case "GET":
			ch.listNodes(w, r)
			return
		}
	case "/1/lb/stats":
		ch.statsGet(w, r)
		return
	case "/1/lb/dash":
		ch.dash(w, r)
		return
	}

	ch.proxy.ServeHTTP(w, r)
}

func (ch *chProxy) statsGet(w http.ResponseWriter, r *http.Request) {
	stats := ch.getStats()

	type st struct {
		Timestamp  time.Time `json:"timestamp"`
		Throughput int       `json:"tp"`
		Node       string    `json:"node"`
	}
	var sts []st

	aggs := make(map[string][]*stat)
	for _, s := range stats {
		// roll up and calculate throughput per second. idk why i hate myself
		if t := aggs[s.node]; len(t) > 0 && t[0].timestamp.Before(s.timestamp.Add(-1*time.Second)) {
			sts = append(sts, st{
				Timestamp:  t[0].timestamp,
				Throughput: len(t),
				Node:       s.node,
			})

			aggs[s.node] = append(aggs[s.node][:0], s)
		} else {
			aggs[s.node] = append(aggs[s.node], s)
		}

	}

	for node, t := range aggs {
		sts = append(sts, st{
			Timestamp:  t[0].timestamp,
			Throughput: len(t),
			Node:       node,
		})
	}

	json.NewEncoder(w).Encode(struct {
		Stats []st `json:"stats"`
	}{
		Stats: sts,
	})
}

func (ch *chProxy) addNode(w http.ResponseWriter, r *http.Request) {
	var bod struct {
		Node string `json:"node"`
	}
	err := json.NewDecoder(r.Body).Decode(&bod)
	if err != nil {
		sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	ch.ch.add(bod.Node)
	sendSuccess(w, "node added")
}

func (ch *chProxy) removeNode(w http.ResponseWriter, r *http.Request) {
	var bod struct {
		Node string `json:"node"`
	}
	err := json.NewDecoder(r.Body).Decode(&bod)
	if err != nil {
		sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	ch.ch.remove(bod.Node)
	sendSuccess(w, "node deleted")
}

func (ch *chProxy) listNodes(w http.ResponseWriter, r *http.Request) {
	nodes := ch.ch.list()
	dead := ch.dead()

	out := make(map[string]string, len(nodes)+len(dead))
	for _, n := range nodes {
		if ch.isDead(n) {
			out[n] = "offline"
		} else {
			out[n] = "online"
		}
	}

	for _, n := range dead {
		out[n] = "offline"
	}

	sendValue(w, struct {
		Nodes map[string]string `json:"nodes"`
	}{
		Nodes: out,
	})
}

func (ch *chProxy) isDead(node string) bool {
	ch.RLock()
	val, ok := ch.ded[node]
	ch.RUnlock()
	return ok && val >= ch.hcUnhealthy
}

func (ch *chProxy) dead() []string {
	ch.RLock()
	defer ch.RUnlock()
	nodes := make([]string, 0, len(ch.ded))
	for n, val := range ch.ded {
		if val >= ch.hcUnhealthy {
			nodes = append(nodes, n)
		}
	}
	return nodes
}

var dashStr = `<!DOCTYPE html>
<html>
<head>
<meta http-equiv="Content-Type" content="text/html; charset=utf-8">
<title>lb dash</title>

<!-- 1. Add these JavaScript inclusions in the head of your page -->
<script type="text/javascript" src="https://code.jquery.com/jquery-1.10.1.js"></script>
<script type="text/javascript" src="https://code.highcharts.com/stock/highstock.js"></script>
<script type="text/javascript" src="https://code.highcharts.com/stock/modules/exporting.js"></script>
<script>
%s
</script>

<!-- 2. Add the JavaScript to initialize the chart on document ready -->
</head>
<body>

<!-- 3. Add the container -->
<div id="container" style="height: 400px; min-width: 310px"></div>

</body>
</html>
`

func (ch *chProxy) dash(w http.ResponseWriter, r *http.Request) {
	w.Write(dashPage)
}

func sendValue(w http.ResponseWriter, v interface{}) {
	err := json.NewEncoder(w).Encode(v)

	if err != nil {
		logrus.WithError(err).Error("error writing response response")
	}
}

func sendSuccess(w http.ResponseWriter, msg string) {
	err := json.NewEncoder(w).Encode(struct {
		Msg string `json:"msg"`
	}{
		Msg: msg,
	})

	if err != nil {
		logrus.WithError(err).Error("error writing response response")
	}
}

func sendError(w http.ResponseWriter, code int, msg string) {
	w.WriteHeader(code)

	err := json.NewEncoder(w).Encode(struct {
		Msg string `json:"msg"`
	}{
		Msg: msg,
	})

	if err != nil {
		logrus.WithError(err).Error("error writing response response")
	}
}
