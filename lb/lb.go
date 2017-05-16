package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/dchest/siphash"
)

// TODO: consistent hashing is nice to get a cheap way to place nodes but it
// doesn't account well for certain functions that may be 'hotter' than others.
// we should very likely keep a load ordered list and distribute based on that.
// if we can get some kind of feedback from the f(x) nodes, we can use that.
// maybe it's good enough to just ch(x) + 1 if ch(x) is marked as "hot"?

// TODO the load balancers all need to have the same list of nodes. gossip?
// also gossip would handle failure detection instead of elb style

// TODO when adding nodes we should health check them once before adding them
// TODO when node goes offline should try to redirect request instead of 5xxing

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

type chProxy struct {
	ch consistentHash

	sync.RWMutex
	// TODO map[string][]time.Time
	ded map[string]int64

	hcInterval  time.Duration
	hcEndpoint  string
	hcUnhealthy int64
	hcTimeout   time.Duration
	proxy       *httputil.ReverseProxy
}

func newProxy(conf config) *chProxy {
	ch := &chProxy{
		ded: make(map[string]int64),

		// XXX (reed): need to be reconfigurable at some point
		hcInterval:  time.Duration(conf.HealthcheckInterval) * time.Second,
		hcEndpoint:  conf.HealthcheckEndpoint,
		hcUnhealthy: int64(conf.HealthcheckUnhealthy),
		hcTimeout:   time.Duration(conf.HealthcheckTimeout) * time.Second,
	}

	director := func(req *http.Request) {
		target := ch.ch.get(req.URL.Path)

		req.URL.Scheme = "http" // XXX (reed): h2 support
		req.URL.Host = target
	}

	ch.proxy = &httputil.ReverseProxy{
		// XXX (reed): optimized http client
		// XXX (reed): buffer pool
		Director: director,
	}

	for _, n := range conf.Nodes {
		// XXX (reed): need to health check these
		ch.ch.add(n)
	}
	go ch.healthcheck()
	return ch
}

func (ch *chProxy) healthcheck() {
	for range time.Tick(ch.hcInterval) {
		nodes := ch.ch.list()
		nodes = append(nodes, ch.dead()...)
		// XXX (reed): need to figure out elegant adding / removing better
		for _, n := range nodes {
			go ch.ping(n)
		}
	}
}

func (ch *chProxy) ping(node string) {
	req, _ := http.NewRequest("GET", "http://"+node+ch.hcEndpoint, nil)
	ctx, cancel := context.WithTimeout(context.Background(), ch.hcTimeout)
	defer cancel()
	req = req.WithContext(ctx)

	// XXX (reed): use same transport as proxy is using
	resp, err := http.DefaultClient.Do(req)
	if resp != nil && resp.Body != nil {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}

	if err != nil || resp.StatusCode < 200 || resp.StatusCode > 299 {
		logrus.WithFields(logrus.Fields{"node": node}).Error("health check failed")
		ch.fail(node)
	} else {
		ch.alive(node)
	}
}

func (ch *chProxy) fail(node string) {
	// shouldn't be a hot path so shouldn't be too contended on since health
	// checks are infrequent
	ch.Lock()
	ch.ded[node]++
	failed := ch.ded[node]
	ch.Unlock()

	if failed >= ch.hcUnhealthy {
		ch.ch.remove(node) // TODO under lock?
	}
}

func (ch *chProxy) alive(node string) {
	ch.RLock()
	_, ok := ch.ded[node]
	ch.RUnlock()
	if ok {
		ch.Lock()
		delete(ch.ded, node)
		ch.Unlock()
		ch.ch.add(node) // TODO under lock?
	}
}

func (ch *chProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/1/lb/nodes" {
		switch r.Method {
		case "PUT":
			// XXX (reed): addNode
			ch.addNode(w, r)
			return
		case "DELETE":
			// XXX (reed): removeNode?
			ch.removeNode(w, r)
			return
		case "GET":
			ch.listNodes(w, r)
			return
		}

		// XXX (reed): stats?

		// XXX (reed): probably do these on a separate port to avoid conflicts
	}

	ch.proxy.ServeHTTP(w, r)
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

// consistentHash will maintain a list of strings which can be accessed by
// keying them with a separate group of strings
type consistentHash struct {
	// protects nodes
	sync.RWMutex
	nodes []string
}

func (ch *consistentHash) add(newb string) {
	ch.Lock()
	defer ch.Unlock()

	// filter dupes, under lock. sorted, so binary search
	i := sort.SearchStrings(ch.nodes, newb)
	if i < len(ch.nodes) && ch.nodes[i] == newb {
		return
	}
	ch.nodes = append(ch.nodes, newb)
	// need to keep in sorted order so that hash index works across nodes
	sort.Sort(sort.StringSlice(ch.nodes))
}

func (ch *consistentHash) remove(ded string) {
	ch.Lock()
	i := sort.SearchStrings(ch.nodes, ded)
	if i < len(ch.nodes) && ch.nodes[i] == ded {
		ch.nodes = append(ch.nodes[:i], ch.nodes[i+1:]...)
	}
	ch.Unlock()
}

// return a copy
func (ch *consistentHash) list() []string {
	ch.RLock()
	ret := make([]string, len(ch.nodes))
	copy(ret, ch.nodes)
	ch.RUnlock()
	return ret
}

func (ch *consistentHash) get(key string) string {
	// crc not unique enough & sha is too slow, it's 1 import
	sum64 := siphash.Hash(0, 0x4c617279426f6174, []byte(key))

	ch.RLock()
	defer ch.RUnlock()
	i := int(jumpConsistentHash(sum64, int32(len(ch.nodes))))
	return ch.nodes[i]
}

// A Fast, Minimal Memory, Consistent Hash Algorithm:
// https://arxiv.org/ftp/arxiv/papers/1406/1406.2294.pdf
func jumpConsistentHash(key uint64, num_buckets int32) int32 {
	var b, j int64 = -1, 0
	for j < int64(num_buckets) {
		b = j
		key = key*2862933555777941757 + 1
		j = (b + 1) * int64((1<<31)/(key>>33)+1)
	}
	return int32(b)
}
