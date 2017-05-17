package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/http/httputil"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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
	ch *consistentHash

	sync.RWMutex
	// TODO map[string][]time.Time
	ded map[string]int64

	hcInterval  time.Duration
	hcEndpoint  string
	hcUnhealthy int64
	hcTimeout   time.Duration

	statMu sync.Mutex
	stats  []*stat

	proxy      *httputil.ReverseProxy
	httpClient *http.Client
	transport  http.RoundTripper
}

type stat struct {
	tim     time.Time
	latency time.Duration
	host    string
	code    uint64
}

func newProxy(conf config) *chProxy {
	tranny := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 120 * time.Second,
		}).Dial,
		MaxIdleConnsPerHost: 512,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig: &tls.Config{
			ClientSessionCache: tls.NewLRUClientSessionCache(4096),
		},
	}

	ch := &chProxy{
		ded: make(map[string]int64),

		// XXX (reed): need to be reconfigurable at some point
		hcInterval:  time.Duration(conf.HealthcheckInterval) * time.Second,
		hcEndpoint:  conf.HealthcheckEndpoint,
		hcUnhealthy: int64(conf.HealthcheckUnhealthy),
		hcTimeout:   time.Duration(conf.HealthcheckTimeout) * time.Second,
		httpClient:  &http.Client{Transport: tranny},
		transport:   tranny,

		ch: newCH(),
	}

	director := func(req *http.Request) {
		target, err := ch.ch.get(req.URL.Path)
		if err != nil {
			target = "error"
		}

		req.URL.Scheme = "http" // XXX (reed): h2 support
		req.URL.Host = target
	}

	ch.proxy = &httputil.ReverseProxy{
		Director:   director,
		Transport:  tranny,
		BufferPool: newBufferPool(),
	}

	for _, n := range conf.Nodes {
		// XXX (reed): need to health check these
		ch.ch.add(n)
	}
	go ch.healthcheck()
	return ch
}

func (ch *chProxy) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host == "error" {
		io.Copy(ioutil.Discard, req.Body)
		req.Body.Close()
		// XXX (reed): if we let the proxy code write the response it will be body-less. ok?
		return nil, ErrNoNodes
	}

	resp, err := ch.transport.RoundTrip(req)
	ch.intercept(req, resp)
	return resp, err
}

func (ch *chProxy) intercept(req *http.Request, resp *http.Response) {
	// XXX (reed): give f(x) nodes ability to send back wait time in response
	// XXX (reed): we should prob clear this from user response
	load, _ := strconv.Atoi(resp.Header.Get("XXX-FXLB-WAIT"))
	// XXX (reed): need to validate these prob
	ch.ch.setLoad(loadKey(req.URL.Host, req.URL.Path), int64(load))

	// XXX (reed): stats data
	//ch.statsMu.Lock()
	//ch.stats = append(ch.stats, &stat{
	//host: r.URL.Host,
	//}
	//ch.stats =  r.URL.Host
}

type bufferPool struct {
	bufs *sync.Pool
}

func newBufferPool() httputil.BufferPool {
	return &bufferPool{
		bufs: &sync.Pool{
			New: func() interface{} { return make([]byte, 32*1024) },
		},
	}
}

func (b *bufferPool) Get() []byte  { return b.bufs.Get().([]byte) }
func (b *bufferPool) Put(x []byte) { b.bufs.Put(x) }

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

	resp, err := ch.httpClient.Do(req)
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
			ch.addNode(w, r)
			return
		case "DELETE":
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

	loadMu sync.RWMutex
	load   map[string]*int64
	rng    *rand.Rand
}

func newCH() *consistentHash {
	return &consistentHash{
		rng:  rand.New(rand.NewSource(time.Now().Unix())),
		load: make(map[string]*int64),
	}
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

func (ch *consistentHash) get(key string) (string, error) {
	// crc not unique enough & sha is too slow, it's 1 import
	sum64 := siphash.Hash(0, 0x4c617279426f6174, []byte(key))

	ch.RLock()
	defer ch.RUnlock()
	i := int(jumpConsistentHash(sum64, int32(len(ch.nodes))))
	return ch.besti(key, i)
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

func (ch *consistentHash) setLoad(key string, load int64) {
	ch.loadMu.RLock()
	l, ok := ch.load[key]
	ch.loadMu.RUnlock()
	if ok {
		atomic.StoreInt64(l, load)
	} else {
		ch.loadMu.Lock()
		if _, ok := ch.load[key]; !ok {
			ch.load[key] = &load
		}
		ch.loadMu.Unlock()
	}
}

var (
	ErrNoNodes = errors.New("no nodes available")
)

func loadKey(node, key string) string {
	return node + "\x00" + key
}

// XXX (reed): push down fails / load into ch
func (ch *consistentHash) besti(key string, i int) (string, error) {
	ch.RLock()
	defer ch.RUnlock()

	if len(ch.nodes) < 1 {
		return "", ErrNoNodes
	}

	f := func(n string) string {
		var load int64
		ch.loadMu.RLock()
		loadPtr := ch.load[loadKey(node, key)]
		ch.loadMu.RUnlock()
		if loadPtr != nil {
			load = atomic.LoadInt64(loadPtr)
		}

		// TODO flesh out these values. should be wait times.
		// if we send < 50% of traffic off to other nodes when loaded
		// then as function scales nodes will get flooded, need to be careful.
		//
		// back off loaded node/function combos slightly to spread load
		if load < 70 {
			return n
		} else if load > 90 {
			if ch.rng.Intn(100) < 60 {
				return n
			}
		} else if load > 70 {
			if ch.rng.Float64() < 80 {
				return n
			}
		}
		// otherwise loop until we find a sufficiently unloaded node or a lucky coin flip
		return ""
	}

	for _, n := range ch.nodes[i:] {
		node := f(n)
		if node != "" {
			return node, nil
		}
	}

	// try the other half of the ring
	for _, n := range ch.nodes[:i] {
		node := f(n)
		if node != "" {
			return node, nil
		}
	}

	return "", ErrNoNodes
}
