package lb

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"fmt"
	"github.com/coreos/go-semver/semver"
	"github.com/sirupsen/logrus"
)

// NewAllGrouper returns a Grouper that will return the entire list of nodes
// that are being maintained, regardless of key.  An 'AllGrouper' will health
// check servers at a specified interval, taking them in and out as they
// pass/fail and exposes endpoints for adding, removing and listing nodes.
func NewAllGrouper(conf Config, db DBStore) (Grouper, error) {
	a := &allGrouper{
		nodeList: make(map[string]*nodeState),
		db:       db,

		// XXX (reed): need to be reconfigurable at some point
		hcInterval:    time.Duration(conf.HealthcheckInterval) * time.Second,
		hcEndpoint:    conf.HealthcheckEndpoint,
		hcUnhealthy:   int64(conf.HealthcheckUnhealthy),
		hcHealthy:     int64(conf.HealthcheckHealthy),
		hcTimeout:     time.Duration(conf.HealthcheckTimeout) * time.Second,
		minAPIVersion: *conf.MinAPIVersion,

		// for health checks
		httpClient: &http.Client{Transport: conf.Transport},
	}

	empty_list := make([]string, 0)
	a.healthyList.Store(&empty_list)

	for _, n := range conf.Nodes {
		err := a.add(n)
		if err != nil {
			// XXX (reed): could prob ignore these but meh
			logrus.WithError(err).WithFields(logrus.Fields{"node": n}).Error("error adding node")
		}
	}
	go a.healthcheck()
	return a, nil
}

// nodeState is used to store success/fail counts and other health related data.
type nodeState struct {
	// name/address of the node
	name string

	// used to purge/delete old nodes when synching the nodes with DB
	intervalID uint64

	// exclusion for multiple go routine pingers
	lock sync.Mutex

	// num of consecutive successes & failures
	success uint64
	fail    uint64

	// current health state
	healthy bool
}

// allGrouper will return all healthy nodes it is tracking from List.
// nodes may be added / removed through the HTTP api. each allGrouper will
// poll its database for the full list of nodes, and then run its own
// health checks on those nodes to maintain a list of healthy nodes.
// the list of healthy nodes will be maintained in sorted order so that,
// without any network partitions, all lbs may consistently hash with the
// same backing list, such that H(k) -> v for any k->v pair (vs attempting
// to maintain a list among nodes in the db, which could have thrashing
// due to network connectivity between any pair).
type allGrouper struct {

	// health checker state and lock
	nodeLock sync.Mutex
	nodeList map[string]*nodeState

	// current atomic store/load protected healthy list
	healthyList atomic.Value

	db DBStore

	httpClient *http.Client

	hcInterval    time.Duration
	hcEndpoint    string
	hcUnhealthy   int64
	hcHealthy     int64
	hcTimeout     time.Duration
	minAPIVersion semver.Version
}

func (a *allGrouper) add(newb string) error {
	if newb == "" {
		return nil // we can't really do a lot of validation since hosts could be an ip or domain but we have health checks
	}
	err := a.checkAPIVersion(newb)
	if err != nil {
		return err
	}
	return a.db.Add(newb)
}

func (a *allGrouper) remove(ded string) error {
	return a.db.Delete(ded)
}

func (a *allGrouper) publishHealth() {

	// get a list of healthy nodes
	a.nodeLock.Lock()
	new_list := make([]string, 0, len(a.nodeList))
	for key, value := range a.nodeList {
		if value.healthy {
			new_list = append(new_list, key)
		}
	}
	a.nodeLock.Unlock()

	// sort and set the healty list pointer
	sort.Strings(new_list)
	a.healthyList.Store(&new_list)
}

// return a copy
func (a *allGrouper) List(string) ([]string, error) {

	// safe atomic load on the current healthy list
	ptr := a.healthyList.Load().(*[]string)
	if len(*ptr) == 0 {
		return nil, ErrNoNodes
	}

	ret := make([]string, len(*ptr))
	copy(ret, *ptr)
	return ret, nil
}

func (a *allGrouper) runHealthCheck(iteration *uint64) {

	// fetch a list of nodes from DB
	list, err := a.db.List()
	if err != nil {
		// if DB fails, the show must go on, report it but perform HC
		logrus.WithError(err).Error("error checking db for nodes")
	} else {
		*iteration++
		isChanged := false
		a.nodeLock.Lock()
		// handle existing & new nodes
		for _, node := range list {
			_, ok := a.nodeList[node]
			if ok {
				// mark existing node
				a.nodeList[node].intervalID = *iteration
			} else {
				// add new node
				a.nodeList[node] = &nodeState{
					name:       node,
					intervalID: *iteration,
					healthy:    true,
				}
				isChanged = true
			}
		}
		// handle deleted nodes: purge unmarked nodes
		for key, value := range a.nodeList {
			if value.intervalID != *iteration {
				delete(a.nodeList, key)
				isChanged = true
			}
		}
		a.nodeLock.Unlock()

		// publish if add/deleted nodes
		if isChanged {
			a.publishHealth()
		}
	}

	// get a list of node pointers
	a.nodeLock.Lock()
	run_list := make([]*nodeState, 0, len(a.nodeList))
	for _, value := range a.nodeList {
		run_list = append(run_list, value)
	}
	a.nodeLock.Unlock()

	for _, node := range run_list {
		go a.ping(node)
	}
}

func (a *allGrouper) healthcheck() {

	// keep track of iteration id in order to mark/sweep deleted nodes
	iteration := uint64(0)

	// run hc immediately upon startup
	a.runHealthCheck(&iteration)

	for range time.Tick(a.hcInterval) {
		a.runHealthCheck(&iteration)
	}
}

type fnVersion struct {
	Version string `json:"version"`
}

func (a *allGrouper) getVersion(urlString string) (string, error) {
	req, _ := http.NewRequest(http.MethodGet, urlString, nil)
	ctx, cancel := context.WithTimeout(context.Background(), a.hcTimeout)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()

	var v fnVersion
	err = json.NewDecoder(resp.Body).Decode(&v)
	if err != nil {
		return "", err
	}
	return v.Version, nil
}

func (a *allGrouper) checkAPIVersion(node string) error {
	versionURL := "http://" + node + a.hcEndpoint

	version, err := a.getVersion(versionURL)
	if err != nil {
		return err
	}

	nodeVer, err := semver.NewVersion(version)
	if err != nil {
		return err
	}

	if nodeVer.LessThan(a.minAPIVersion) {
		return fmt.Errorf("incompatible API version: %v", nodeVer)
	}
	return nil
}

func (a *allGrouper) ping(node *nodeState) {
	err := a.checkAPIVersion(node.name)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{"node": node.name}).Error("Unable to check API version")
		a.fail(node)
	} else {
		a.alive(node)
	}
}

func (a *allGrouper) fail(node *nodeState) {

	isChanged := false

	node.lock.Lock()

	node.success = 0
	node.fail++

	if node.healthy && node.fail >= uint64(a.hcUnhealthy) {
		node.healthy = false
		isChanged = true
	}

	node.lock.Unlock()

	if isChanged {
		logrus.WithFields(logrus.Fields{"node": node.name}).Info("is unhealthy")
		a.publishHealth()
	}
}

func (a *allGrouper) alive(node *nodeState) {

	isChanged := false

	node.lock.Lock()

	node.fail = 0
	node.success++

	if !node.healthy && node.success >= uint64(a.hcHealthy) {
		node.healthy = true
		isChanged = true
	}

	node.lock.Unlock()

	if isChanged {
		logrus.WithFields(logrus.Fields{"node": node.name}).Info("is healthy")
		a.publishHealth()
	}
}

func (a *allGrouper) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/1/lb/nodes":
			switch r.Method {
			case "PUT":
				a.addNode(w, r)
			case "DELETE":
				a.removeNode(w, r)
			case "GET":
				a.listNodes(w, r)
			}
			return
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

	err = a.add(bod.Node)
	if err != nil {
		sendError(w, 500, err.Error()) // TODO filter ?
		return
	}
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

	err = a.remove(bod.Node)
	if err != nil {
		sendError(w, 500, err.Error()) // TODO filter ?
		return
	}
	sendSuccess(w, "node deleted")
}

func (a *allGrouper) listNodes(w http.ResponseWriter, r *http.Request) {

	// get a list of node pointers
	a.nodeLock.Lock()
	run_list := make([]*nodeState, 0, len(a.nodeList))
	for _, value := range a.nodeList {
		run_list = append(run_list, value)
	}
	a.nodeLock.Unlock()

	out := make(map[string]string, len(run_list))
	for _, node := range run_list {

		node.lock.Lock()
		healthy := node.healthy
		node.lock.Unlock()

		if healthy {
			out[node.name] = "online"
		} else {
			out[node.name] = "offline"
		}
	}

	sendValue(w, struct {
		Nodes map[string]string `json:"nodes"`
	}{
		Nodes: out,
	})
}
