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

	"fmt"
	"github.com/coreos/go-semver/semver"
	"github.com/sirupsen/logrus"
)

const AllGrouperDriver = "rest"

// NewAllGrouper returns a Grouper that will return the entire list of nodes
// that are being maintained, regardless of key.  An 'AllGrouper' will health
// check servers at a specified interval, taking them in and out as they
// pass/fail and exposes endpoints for adding, removing and listing nodes.
func NewAllGrouper(conf Config, db DBStore) (Grouper, error) {
	a := &allGrouper{
		nodeList:        make(map[string]nodeState),
		nodeHealthyList: make([]string, 0),
		db:              db,

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

type healthState uint8

const (
	StateUnknown = iota
	StateHealthy
	StateUnhealthy
)

// nodeState is used to store success/fail counts and other health related data.
type nodeState struct {

	// num of consecutive successes & failures
	success uint64
	fail    uint64

	// current health state
	healthy healthState
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
	nodeLock        sync.RWMutex
	nodeList        map[string]nodeState
	nodeHealthyList []string

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
	return a.db.Add(newb)
}

func (a *allGrouper) remove(ded string) error {
	return a.db.Delete(ded)
}

func (a *allGrouper) publishHealth() {

	a.nodeLock.Lock()

	// get a list of healthy nodes
	newList := make([]string, 0, len(a.nodeList))
	for key, value := range a.nodeList {
		if value.healthy == StateHealthy {
			newList = append(newList, key)
		}
	}

	// sort and update healthy List
	sort.Strings(newList)
	a.nodeHealthyList = newList

	a.nodeLock.Unlock()
}

// return a copy
func (a *allGrouper) List(string) ([]string, error) {

	a.nodeLock.RLock()
	ret := make([]string, len(a.nodeHealthyList))
	copy(ret, a.nodeHealthyList)
	a.nodeLock.RUnlock()
	return ret, nil
}

func (a *allGrouper) runHealthCheck() {

	// fetch a list of nodes from DB
	list, err := a.db.List()
	if err != nil {
		// if DB fails, the show must go on, report it but perform HC
		logrus.WithError(err).Error("error checking db for nodes")

		// compile a list of nodes to be health checked
		a.nodeLock.RLock()
		list = make([]string, 0, len(a.nodeList))
		for key, _ := range a.nodeList {
			list = append(list, key)
		}
		a.nodeLock.RUnlock()

	} else {

		isChanged := false

		// compile a map of DB nodes for deletion check
		deleteCheck := make(map[string]bool, len(list))
		for _, node := range list {
			deleteCheck[node] = true
		}

		a.nodeLock.Lock()

		// handle new nodes
		for _, node := range list {
			_, ok := a.nodeList[node]
			if !ok {
				// add new node
				a.nodeList[node] = nodeState{healthy: StateUnknown}
				isChanged = true
			}
		}

		// handle deleted nodes: purge unmarked nodes
		for key, _ := range a.nodeList {
			_, ok := deleteCheck[key]
			if !ok {
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

	// spawn health checkers
	for _, key := range list {
		go a.ping(key)
	}
}

func (a *allGrouper) healthcheck() {

	// run hc immediately upon startup
	a.runHealthCheck()

	for range time.Tick(a.hcInterval) {
		a.runHealthCheck()
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

func (a *allGrouper) ping(node string) {
	err := a.checkAPIVersion(node)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{"node": node}).Error("Unable to check API version")
		a.fail(node)
	} else {
		a.alive(node)
	}
}

func (a *allGrouper) fail(key string) {

	isChanged := false

	a.nodeLock.Lock()

	// if deleted, skip
	node, ok := a.nodeList[key]
	if !ok {
		a.nodeLock.Unlock()
		return
	}

	node.success = 0
	node.fail++

	// overflow case
	if node.fail == 0 {
		node.fail = uint64(a.hcUnhealthy)
	}

	if (node.healthy == StateHealthy && node.fail >= uint64(a.hcUnhealthy)) || node.healthy == StateUnknown {
		node.healthy = StateUnhealthy
		isChanged = true
	}

	a.nodeList[key] = node
	a.nodeLock.Unlock()

	if isChanged {
		logrus.WithFields(logrus.Fields{"node": key}).Info("is unhealthy")
		a.publishHealth()
	}
}

func (a *allGrouper) alive(key string) {

	isChanged := false

	a.nodeLock.Lock()

	// if deleted, skip
	node, ok := a.nodeList[key]
	if !ok {
		a.nodeLock.Unlock()
		return
	}

	node.fail = 0
	node.success++

	// overflow case
	if node.success == 0 {
		node.success = uint64(a.hcHealthy)
	}

	if (node.healthy == StateUnhealthy && node.success >= uint64(a.hcHealthy)) || node.healthy == StateUnknown {
		node.healthy = StateHealthy
		isChanged = true
	}

	a.nodeList[key] = node
	a.nodeLock.Unlock()

	if isChanged {
		logrus.WithFields(logrus.Fields{"node": key}).Info("is healthy")
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

	a.nodeLock.RLock()

	out := make(map[string]string, len(a.nodeList))

	for key, value := range a.nodeList {
		if value.healthy == StateHealthy {
			out[key] = "online"
		} else {
			out[key] = "offline"
		}
	}

	a.nodeLock.RUnlock()

	sendValue(w, struct {
		Nodes map[string]string `json:"nodes"`
	}{
		Nodes: out,
	})
}
