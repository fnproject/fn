package lb

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
)

// NewAllGrouper returns a Grouper that will return the entire list of nodes
// that are being maintained, regardless of key.  An 'AllGrouper' will health
// check servers at a specified interval, taking them in and out as they
// pass/fail and exposes endpoints for adding, removing and listing nodes.
func NewAllGrouper(conf Config) Grouper {
	a := &allGrouper{
		ded: make(map[string]int64),

		// XXX (reed): need to be reconfigurable at some point
		hcInterval:  time.Duration(conf.HealthcheckInterval) * time.Second,
		hcEndpoint:  conf.HealthcheckEndpoint,
		hcUnhealthy: int64(conf.HealthcheckUnhealthy),
		hcTimeout:   time.Duration(conf.HealthcheckTimeout) * time.Second,

		// for health checks
		httpClient: &http.Client{Transport: conf.Transport},
	}
	for _, n := range conf.Nodes {
		a.add(n)
	}
	go a.healthcheck()
	return a
}

// TODO
type allGrouper struct {
	// protects nodes & ded
	sync.RWMutex
	nodes []string
	ded   map[string]int64 // [node] -> failedCount

	httpClient *http.Client

	hcInterval  time.Duration
	hcEndpoint  string
	hcUnhealthy int64
	hcTimeout   time.Duration
}

func (a *allGrouper) add(newb string) {
	if newb == "" {
		return // we can't really do a lot of validation since hosts could be an ip or domain but we have health checks
	}
	a.Lock()
	a.addNoLock(newb)
	a.Unlock()
}

func (a *allGrouper) addNoLock(newb string) {
	// filter dupes, under lock. sorted, so binary search
	i := sort.SearchStrings(a.nodes, newb)
	if i < len(a.nodes) && a.nodes[i] == newb {
		return
	}
	a.nodes = append(a.nodes, newb)
	// need to keep in sorted order so that hash index works across nodes
	sort.Sort(sort.StringSlice(a.nodes))
}

func (a *allGrouper) remove(ded string) {
	a.Lock()
	a.removeNoLock(ded)
	a.Unlock()
}

func (a *allGrouper) removeNoLock(ded string) {
	i := sort.SearchStrings(a.nodes, ded)
	if i < len(a.nodes) && a.nodes[i] == ded {
		a.nodes = append(a.nodes[:i], a.nodes[i+1:]...)
	}
}

// return a copy
func (a *allGrouper) List(string) ([]string, error) {
	a.RLock()
	ret := make([]string, len(a.nodes))
	copy(ret, a.nodes)
	a.RUnlock()
	var err error
	if len(ret) == 0 {
		err = ErrNoNodes
	}
	return ret, err
}

func (a *allGrouper) healthcheck() {
	for range time.Tick(a.hcInterval) {
		nodes, _ := a.List("")
		nodes = append(nodes, a.dead()...)
		for _, n := range nodes {
			go a.ping(n)
		}
	}
}

func (a *allGrouper) ping(node string) {
	req, _ := http.NewRequest("GET", "http://"+node+a.hcEndpoint, nil)
	ctx, cancel := context.WithTimeout(context.Background(), a.hcTimeout)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := a.httpClient.Do(req)
	if resp != nil && resp.Body != nil {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}

	if err != nil || resp.StatusCode < 200 || resp.StatusCode > 299 {
		logrus.WithError(err).WithFields(logrus.Fields{"node": node}).Error("health check failed")
		a.fail(node)
	} else {
		a.alive(node)
	}
}

func (a *allGrouper) fail(node string) {
	// shouldn't be a hot path so shouldn't be too contended on since health
	// checks are infrequent
	a.Lock()
	a.ded[node]++
	failed := a.ded[node]
	if failed >= a.hcUnhealthy {
		a.removeNoLock(node)
	}
	a.Unlock()
}

func (a *allGrouper) alive(node string) {
	a.RLock()
	_, ok := a.ded[node]
	a.RUnlock()
	if ok {
		a.Lock()
		delete(a.ded, node)
		a.addNoLock(node)
		a.Unlock()
	}
}

func (a *allGrouper) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		// XXX (reed): probably do these on a separate port to avoid conflicts
		case "/1/lb/nodes":
			switch r.Method {
			case "PUT":
				a.addNode(w, r)
				return
			case "DELETE":
				a.removeNode(w, r)
				return
			case "GET":
				a.listNodes(w, r)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func (a *allGrouper) addNode(w http.ResponseWriter, r *http.Request) {
	var bod struct {
		Node string `json:"node"`
	}
	err := json.NewDecoder(r.Body).Decode(&bod)
	if err != nil {
		sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	a.add(bod.Node)
	sendSuccess(w, "node added")
}

func (a *allGrouper) removeNode(w http.ResponseWriter, r *http.Request) {
	var bod struct {
		Node string `json:"node"`
	}
	err := json.NewDecoder(r.Body).Decode(&bod)
	if err != nil {
		sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	a.remove(bod.Node)
	sendSuccess(w, "node deleted")
}

func (a *allGrouper) listNodes(w http.ResponseWriter, r *http.Request) {
	nodes, _ := a.List("")
	dead := a.dead()

	out := make(map[string]string, len(nodes)+len(dead))
	for _, n := range nodes {
		if a.isDead(n) {
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

func (a *allGrouper) isDead(node string) bool {
	a.RLock()
	val, ok := a.ded[node]
	a.RUnlock()
	return ok && val >= a.hcUnhealthy
}

func (a *allGrouper) dead() []string {
	a.RLock()
	defer a.RUnlock()
	nodes := make([]string, 0, len(a.ded))
	for n, val := range a.ded {
		if val >= a.hcUnhealthy {
			nodes = append(nodes, n)
		}
	}
	return nodes
}
