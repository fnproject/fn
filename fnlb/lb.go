package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
)

// TODO the load balancers all need to have the same list of nodes. gossip?
// also gossip would handle failure detection instead of elb style. or it can
// be pluggable and then we can read from where bmc is storing them and use that
// or some OSS alternative

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
	flag.StringVar(&conf.Listen, "listen", ":8081", "port to run on")
	flag.IntVar(&conf.HealthcheckInterval, "hc-interval", 3, "how often to check f(x) nodes, in seconds")
	flag.StringVar(&conf.HealthcheckEndpoint, "hc-path", "/version", "endpoint to determine node health")
	flag.IntVar(&conf.HealthcheckUnhealthy, "hc-unhealthy", 2, "threshold of failed checks to declare node unhealthy")
	flag.IntVar(&conf.HealthcheckTimeout, "hc-timeout", 5, "timeout of healthcheck endpoint, in seconds")
	flag.Parse()

	conf.Nodes = strings.Split(*fnodes, ",")

	ch := newProxy(conf)

	err := serve(conf.Listen, ch)
	if err != nil {
		logrus.WithError(err).Error("server error")
	}
}

func serve(addr string, handler http.Handler) error {
	server := &http.Server{Addr: addr, Handler: handler}

	var wg sync.WaitGroup
	wg.Add(1)
	defer wg.Wait()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGQUIT, syscall.SIGINT)
	go func() {
		defer wg.Done()
		for sig := range ch {
			logrus.WithFields(logrus.Fields{"signal": sig}).Info("received signal")
			server.Shutdown(context.Background()) // safe shutdown
			return
		}
	}()
	return server.ListenAndServe()
}

type config struct {
	Listen               string   `json:"port"`
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
		Func       string    `json:"func"`
		Wait       float64   `json:"wait"` // seconds
	}
	var sts []st

	// roll up and calculate throughput per second. idk why i hate myself
	aggs := make(map[string][]*stat)
	for _, s := range stats {
		key := s.node + "/" + s.fx
		if t := aggs[key]; len(t) > 0 && t[0].timestamp.Before(s.timestamp.Add(-1*time.Second)) {
			sts = append(sts, st{
				Timestamp:  t[0].timestamp,
				Throughput: len(t),
				Node:       t[0].node,
				Func:       t[0].fx,
				Wait:       avgWait(t),
			})

			aggs[key] = append(aggs[key][:0], s)
		} else {
			aggs[key] = append(aggs[key], s)
		}
	}

	// leftovers
	for _, t := range aggs {
		sts = append(sts, st{
			Timestamp:  t[0].timestamp,
			Throughput: len(t),
			Node:       t[0].node,
			Func:       t[0].fx,
			Wait:       avgWait(t),
		})
	}

	json.NewEncoder(w).Encode(struct {
		Stats []st `json:"stats"`
	}{
		Stats: sts,
	})
}

func avgWait(stats []*stat) float64 {
	var sum time.Duration
	for _, s := range stats {
		sum += s.wait
	}
	return (sum / time.Duration(len(stats))).Seconds()
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

<script type="text/javascript" src="https://code.jquery.com/jquery-1.10.1.js"></script>
<script type="text/javascript" src="https://code.highcharts.com/stock/highstock.js"></script>
<script type="text/javascript" src="https://code.highcharts.com/stock/modules/exporting.js"></script>
<script>
%s
</script>

</head>
<body>

<div id="throughput_chart" style="height: 400px; min-width: 310px"></div>
<div id="wait_chart" style="height: 400px; min-width: 310px"></div>

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
