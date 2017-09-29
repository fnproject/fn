package lb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"fmt"
	"github.com/coreos/go-semver/semver"
	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
)

// NewAllGrouper returns a Grouper that will return the entire list of nodes
// that are being maintained, regardless of key.  An 'AllGrouper' will health
// check servers at a specified interval, taking them in and out as they
// pass/fail and exposes endpoints for adding, removing and listing nodes.
func NewAllGrouper(conf Config) (Grouper, error) {
	db, err := db(conf.DBurl)
	if err != nil {
		return nil, err
	}

	a := &allGrouper{
		ded: make(map[string]int64),
		db:  db,

		// XXX (reed): need to be reconfigurable at some point
		hcInterval:    time.Duration(conf.HealthcheckInterval) * time.Second,
		hcEndpoint:    conf.HealthcheckEndpoint,
		hcUnhealthy:   int64(conf.HealthcheckUnhealthy),
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
	// protects allNodes, healthy & ded
	sync.RWMutex
	// TODO rename nodes to 'allNodes' or something so everything breaks and then stitch
	// ded is the set of disjoint nodes nodes from intersecting nodes & healthy
	allNodes, healthy []string
	ded               map[string]int64 // [node] -> failedCount

	// allNodes is a cache of db.List, we can probably trash it..
	db DBStore

	httpClient *http.Client

	hcInterval    time.Duration
	hcEndpoint    string
	hcUnhealthy   int64
	hcTimeout     time.Duration
	minAPIVersion semver.Version
}

// TODO put this somewhere better
type DBStore interface {
	Add(string) error
	Delete(string) error
	List() ([]string, error)
}

// implements DBStore
type sqlStore struct {
	db *sqlx.DB

	// TODO we should prepare all of the statements, rebind them
	// and store them all here.
}

// New will open the db specified by url, create any tables necessary
// and return a models.Datastore safe for concurrent usage.
func db(uri string) (DBStore, error) {
	url, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	driver := url.Scheme
	// driver must be one of these for sqlx to work, double check:
	switch driver {
	case "postgres", "pgx", "mysql", "sqlite3", "oci8", "ora", "goracle":
	default:
		return nil, errors.New("invalid db driver, refer to the code")
	}

	if driver == "sqlite3" {
		// make all the dirs so we can make the file..
		dir := filepath.Dir(url.Path)
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return nil, err
		}
	}

	uri = url.String()
	if driver != "postgres" {
		// postgres seems to need this as a prefix in lib/pq, everyone else wants it stripped of scheme
		uri = strings.TrimPrefix(url.String(), url.Scheme+"://")
	}

	sqldb, err := sql.Open(driver, uri)
	if err != nil {
		logrus.WithFields(logrus.Fields{"url": uri}).WithError(err).Error("couldn't open db")
		return nil, err
	}

	db := sqlx.NewDb(sqldb, driver)
	// force a connection and test that it worked
	err = db.Ping()
	if err != nil {
		logrus.WithFields(logrus.Fields{"url": uri}).WithError(err).Error("couldn't ping db")
		return nil, err
	}

	maxIdleConns := 30 // c.MaxIdleConnections
	db.SetMaxIdleConns(maxIdleConns)
	logrus.WithFields(logrus.Fields{"max_idle_connections": maxIdleConns, "datastore": driver}).Info("datastore dialed")

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS lb_nodes (
		address text NOT NULL PRIMARY KEY
	);`)
	if err != nil {
		return nil, err
	}

	return &sqlStore{db: db}, nil
}

func (s *sqlStore) Add(node string) error {
	query := s.db.Rebind("INSERT INTO lb_nodes (address) VALUES (?);")
	_, err := s.db.Exec(query, node)
	if err != nil {
		// if it already exists, just filter that error out
		switch err := err.(type) {
		case *mysql.MySQLError:
			if err.Number == 1062 {
				return nil
			}
		case *pq.Error:
			if err.Code == "23505" {
				return nil
			}
		case sqlite3.Error:
			if err.ExtendedCode == sqlite3.ErrConstraintUnique || err.ExtendedCode == sqlite3.ErrConstraintPrimaryKey {
				return nil
			}
		}
	}
	return err
}

func (s *sqlStore) Delete(node string) error {
	query := s.db.Rebind(`DELETE FROM lb_nodes WHERE address=?`)
	_, err := s.db.Exec(query, node)
	// TODO we can filter if it didn't exist, too...
	return err
}

func (s *sqlStore) List() ([]string, error) {
	query := s.db.Rebind(`SELECT DISTINCT address FROM lb_nodes`)
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}

	var nodes []string
	for rows.Next() {
		var node string
		err := rows.Scan(&node)
		if err == nil {
			nodes = append(nodes, node)
		}
	}

	err = rows.Err()
	if err == sql.ErrNoRows {
		err = nil // don't care...
	}

	return nodes, err
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

// call with a.Lock held
func (a *allGrouper) addHealthy(newb string) {
	// filter dupes, under lock. sorted, so binary search
	i := sort.SearchStrings(a.healthy, newb)
	if i < len(a.healthy) && a.healthy[i] == newb {
		return
	}
	a.healthy = append(a.healthy, newb)
	// need to keep in sorted order so that hash index works across nodes
	sort.Sort(sort.StringSlice(a.healthy))
}

// call with a.Lock held
func (a *allGrouper) removeHealthy(ded string) {
	i := sort.SearchStrings(a.healthy, ded)
	if i < len(a.healthy) && a.healthy[i] == ded {
		a.healthy = append(a.healthy[:i], a.healthy[i+1:]...)
	}
}

// return a copy
func (a *allGrouper) List(string) ([]string, error) {
	a.RLock()
	ret := make([]string, len(a.healthy))
	copy(ret, a.healthy)
	a.RUnlock()
	var err error
	if len(ret) == 0 {
		err = ErrNoNodes
	}
	return ret, err
}

func (a *allGrouper) healthcheck() {
	for range time.Tick(a.hcInterval) {
		// health check the entire list of nodes [from db]
		list, err := a.db.List()
		if err != nil {
			logrus.WithError(err).Error("error checking db for nodes")
			continue
		}

		a.Lock()
		a.allNodes = list
		a.Unlock()

		for _, n := range list {
			go a.ping(n)
		}
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
	versionURL := "http://" + node + "/version"

	version, err := a.getVersion(versionURL)
	if err != nil {
		return err
	}

	nodeVer := semver.New(version)
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

func (a *allGrouper) fail(node string) {
	// shouldn't be a hot path so shouldn't be too contended on since health
	// checks are infrequent
	a.Lock()
	a.ded[node]++
	failed := a.ded[node]
	if failed >= a.hcUnhealthy {
		a.removeHealthy(node)
	}
	a.Unlock()
}

func (a *allGrouper) alive(node string) {
	// TODO alive is gonna get called a lot, should maybe start w/ every node in ded
	// so we can RLock (but lock contention should be low since these are ~quick) --
	// "a lot" being every 1s per node, so not too crazy really, but 1k nodes @ ms each...
	a.Lock()
	delete(a.ded, node)
	a.addHealthy(node)
	a.Unlock()
}

func (a *allGrouper) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		// XXX (reed): probably do these on a separate port to avoid conflicts
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
	a.RLock()
	nodes := make([]string, len(a.allNodes))
	copy(nodes, a.allNodes)
	a.RUnlock()

	// TODO this isn't correct until at least one health check has hit all nodes (on start up).
	// seems like not a huge deal, but here's a note anyway (every node will simply 'appear' healthy
	// from this api even if we aren't routing to it [until first health check]).
	out := make(map[string]string, len(nodes))
	for _, n := range nodes {
		if a.isDead(n) {
			out[n] = "offline"
		} else {
			out[n] = "online"
		}
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
