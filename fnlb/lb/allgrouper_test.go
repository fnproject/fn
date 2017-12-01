package lb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"sort"
	"testing"
	"time"

	"github.com/coreos/go-semver/semver"
)

type mockDB struct {
	isAddError    bool
	isDeleteError bool
	isListError   bool
	nodeList      map[string]bool
}

func (mock *mockDB) Add(node string) error {
	if mock.isAddError {
		return errors.New("simulated add error")
	}
	mock.nodeList[node] = true
	return nil
}
func (mock *mockDB) Delete(node string) error {
	if mock.isDeleteError {
		return errors.New("simulated delete error")
	}
	delete(mock.nodeList, node)
	return nil
}
func (mock *mockDB) List() ([]string, error) {
	if mock.isListError {
		return nil, errors.New("simulated list error")
	}
	list := make([]string, 0, len(mock.nodeList))
	for key, _ := range mock.nodeList {
		list = append(list, key)
	}
	return list, nil
}

func initializeRunner() (Grouper, error) {
	db := &mockDB{
		nodeList: make(map[string]bool),
	}

	conf := Config{
		HealthcheckInterval:  1,
		HealthcheckEndpoint:  "/version",
		HealthcheckUnhealthy: 1,
		HealthcheckHealthy:   1,
		HealthcheckTimeout:   1,
		MinAPIVersion:        semver.New("0.0.104"),
		Transport:            &http.Transport{},
	}

	return NewAllGrouper(conf, db)
}

type testServer struct {
	addr     string
	version  string
	healthy  bool
	inPool   bool
	listener *net.Listener
	server   *http.Server
}

func (s *testServer) getHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "/version" == r.URL.Path {
			if s.healthy {
				sendValue(w, fnVersion{Version: s.version})
			} else {
				sendError(w, http.StatusServiceUnavailable, "service unhealthy")
			}
		} else {
			sendError(w, http.StatusNotFound, "unknown uri")
		}
	})
}

// return a list of supposed to be healthy (good version and in pool) nodes
func getCurrentHealthySet(list []*testServer) []string {

	out := make([]string, 0)

	for _, val := range list {
		if val.healthy && val.inPool {
			out = append(out, val.addr)
		}
	}

	sort.Strings(out)
	return out
}

// shutdown a server
func teardownServer(t *testing.T, server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(100)*time.Millisecond)
	defer cancel()

	err := server.Shutdown(ctx)
	if err != nil {
		t.Logf("shutdown error: %s", err.Error())
	}
}

// spin up backend servers
func initializeAPIServer(t *testing.T, grouper Grouper) (*http.Server, string, error) {

	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, "", err
	}

	addr := listener.Addr().String()
	handler := NullHandler()
	handler = grouper.Wrap(handler) // add/del/list endpoints
	server := &http.Server{Handler: handler}

	go func(srv *http.Server, addr string) {
		err := server.Serve(listener)
		if err != nil {
			t.Logf("server exited %s with %s", addr, err.Error())
		}
	}(server, addr)

	return server, addr, nil
}

// spin up backend servers
func initializeTestServers(t *testing.T, numOfServers uint64) ([]*testServer, error) {

	list := make([]*testServer, 0)

	for i := uint64(0); i < numOfServers; i++ {

		listener, err := net.Listen("tcp", "localhost:0")
		if err != nil {
			return list, err
		}

		server := &testServer{
			addr:     listener.Addr().String(),
			version:  "0.0.104",
			healthy:  true,
			inPool:   false,
			listener: &listener,
		}

		server.server = &http.Server{Handler: server.getHandler()}

		go func(srv *testServer) {
			err := srv.server.Serve(listener)
			if err != nil {
				t.Logf("server exited %s with %s", srv.addr, err.Error())
			}
		}(server)

		list = append(list, server)
	}

	return list, nil
}

// tear down backend servers
func shutdownTestServers(t *testing.T, servers []*testServer) {
	for _, srv := range servers {
		teardownServer(t, srv.server)
	}
}

func testCompare(t *testing.T, grouper Grouper, servers []*testServer, ctx string) {

	// compare current supposed to be healthy VS healthy list from allGrouper
	current := getCurrentHealthySet(servers)
	t.Logf("%s Expecting healthy servers %v", ctx, current)

	round, err := grouper.List("ignore")
	if err != nil {
		if len(current) != 0 {
			t.Errorf("%s Not expected error %s", ctx, err.Error())
		}
	} else {
		t.Logf("%s Detected healthy servers  %v", ctx, round)

		if len(current) != len(round) {
			t.Errorf("%s Got %d servers, expected: %d", ctx, len(round), len(current))
		}
		for idx, srv := range round {
			if srv != current[idx] {
				t.Errorf("%s Mismatch idx: %d %s != %s", ctx, idx, srv, current[idx])
			}
		}
	}
}

// using mgmt API modify (add/remove) a node
func mgmtModServer(t *testing.T, addr string, operation string, node string) error {
	client := &http.Client{}
	url := "http://" + addr + "/1/lb/nodes"

	str := fmt.Sprintf("{\"Node\":\"%s\"}", node)
	body := []byte(str)
	req, err := http.NewRequest(operation, url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	t.Logf("%s node=%s response=%s status=%d", operation, node, respBody, resp.StatusCode)

	if resp.StatusCode != 200 {
		return fmt.Errorf("%s node=%s status=%d %s", operation, node, resp.StatusCode, respBody)
	}

	return nil
}

// using mgmt api list servers and compare with test server list
func mgmtListServers(t *testing.T, addr string, servers []*testServer) error {
	client := &http.Client{}
	url := "http://" + addr + "/1/lb/nodes"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("list status=%d", resp.StatusCode)
	}

	var listResp struct {
		Nodes map[string]string `json:"nodes"`
	}

	err = json.NewDecoder(resp.Body).Decode(&listResp)
	if err != nil {
		return err
	}

	cmpList := make(map[string]string, len(servers))
	for _, val := range servers {
		if val.inPool {
			if val.healthy {
				cmpList[val.addr] = "online"
			} else {
				cmpList[val.addr] = "offline"
			}
		}
	}

	t.Logf("list response=%v expected=%v", listResp.Nodes, cmpList)

	for key, val1 := range cmpList {
		val2, ok := listResp.Nodes[key]
		if !ok {
			t.Errorf("failed list comparison node=`%s` is not in received", key)
			return nil

		}

		if val1 != val2 {
			t.Errorf("failed list comparison node=`%s` expected=`%s` received=`%s`", key, val1, val2)
			return nil
		}

		delete(cmpList, key)
		delete(listResp.Nodes, key)
	}

	if len(cmpList) != 0 || len(listResp.Nodes) != 0 {
		t.Errorf("failed list comparison (remaining unmatches) expected=`%v` received=`%v`", cmpList, listResp.Nodes)
	}

	return nil
}

// Basic tests via DB add/remove functions
func TestRouteRunnerExecution(t *testing.T) {

	a, err := initializeRunner()
	if err != nil {
		t.Errorf("Not expected error `%s`", err.Error())
	}

	var concrete *allGrouper
	concrete = a.(*allGrouper)

	// initialize and add some servers (all healthy)
	serverCount := 10
	servers, err := initializeTestServers(t, uint64(serverCount))
	if err != nil {
		t.Errorf("Not expected error `%s`", err.Error())
	} else {
		defer shutdownTestServers(t, servers)

		if serverCount != len(servers) {
			t.Errorf("Got %d servers, expected: %d", len(servers), serverCount)
		}

		srvList := make([]string, 0, len(servers))
		for _, srv := range servers {
			srvList = append(srvList, srv.addr)
		}
		t.Logf("Spawned servers %s", srvList)

		testCompare(t, a, servers, "round0")

		for _, srv := range servers {
			// add these servers to allGrouper
			err := concrete.add(srv.addr)
			if err != nil {
				t.Errorf("Not expected error `%s` when adding `%s`", err.Error(), srv.addr)
			}
			srv.inPool = true
		}
	}

	t.Logf("Starting round1 all servers healthy")

	// let health checker converge
	time.Sleep(time.Duration(2) * time.Second)
	testCompare(t, a, servers, "round1")

	t.Logf("Starting round2 one server unhealthy")

	// now set one server unhealthy
	servers[2].healthy = false
	t.Logf("Setting server %s to unhealthy", servers[2].addr)

	// let health checker converge
	time.Sleep(time.Duration(2) * time.Second)
	testCompare(t, a, servers, "round2")

	t.Logf("Starting round3 remove one server from grouper")

	t.Logf("Removing server %s from grouper", servers[3].addr)
	err = concrete.remove(servers[3].addr)
	if err != nil {
		t.Errorf("Not expected error `%s` when removing `%s`", err.Error(), servers[3].addr)
	}
	servers[3].inPool = false

	time.Sleep(time.Duration(2) * time.Second)
	testCompare(t, a, servers, "round3")

	t.Logf("Starting round4 add server back to grouper")

	t.Logf("Adding server %s to grouper", servers[3].addr)
	err = concrete.add(servers[3].addr)
	if err != nil {
		t.Errorf("Not expected error `%s` when adding `%s`", err.Error(), servers[3].addr)
	}
	servers[3].inPool = true

	time.Sleep(time.Duration(2) * time.Second)
	testCompare(t, a, servers, "round4")

	t.Logf("Starting round5 set unhealthy server back to healthy")
	servers[2].healthy = true
	t.Logf("Setting server %s to healthy", servers[2].addr)

	// let health checker converge
	time.Sleep(time.Duration(2) * time.Second)
	testCompare(t, a, servers, "round5")

	t.Logf("Starting round6 no change")
	// fetch list again
	testCompare(t, a, servers, "round6")
}

// Basic tests via mgmt API
func TestRouteRunnerMgmtAPI(t *testing.T) {

	a, err := initializeRunner()
	if err != nil {
		t.Errorf("Not expected error `%s`", err.Error())
	}

	mgmtSrv, mgmtAddr, err := initializeAPIServer(t, a)
	if err != nil {
		t.Errorf("cannot start mgmt api server `%s`", err.Error())
	}
	defer teardownServer(t, mgmtSrv)

	// initialize and add some servers (all healthy)
	serverCount := 5
	servers, err := initializeTestServers(t, uint64(serverCount))
	if err != nil {
		t.Errorf("Not expected error `%s`", err.Error())
	} else {
		defer shutdownTestServers(t, servers)

		if serverCount != len(servers) {
			t.Errorf("Got %d servers, expected: %d", len(servers), serverCount)
		}

		srvList := make([]string, 0, len(servers))
		for _, srv := range servers {
			srvList = append(srvList, srv.addr)
		}
		t.Logf("Spawned servers %s", srvList)

		testCompare(t, a, servers, "round0")

		for _, srv := range servers {
			err := mgmtModServer(t, mgmtAddr, "PUT", srv.addr)
			if err != nil {
				t.Errorf("Not expected error `%s` when adding `%s`", err.Error(), srv.addr)
			}
			srv.inPool = true
		}
	}

	t.Logf("Starting round1 all servers healthy")

	// let health checker converge
	time.Sleep(time.Duration(2) * time.Second)
	testCompare(t, a, servers, "round1")

	err = mgmtListServers(t, mgmtAddr, servers)
	if err != nil {
		t.Errorf("Not expected error `%s` when listing", err.Error())
	}

	t.Logf("Starting round2 remove one server from grouper")

	// let's set server at 2 as unhealthy as well
	servers[2].healthy = false

	t.Logf("Removing server %s from grouper", servers[3].addr)
	err = mgmtModServer(t, mgmtAddr, "DELETE", servers[3].addr)
	if err != nil {
		t.Errorf("Not expected error `%s` when removing `%s`", err.Error(), servers[3].addr)
	}
	servers[3].inPool = false

	time.Sleep(time.Duration(2) * time.Second)
	testCompare(t, a, servers, "round2")

	err = mgmtListServers(t, mgmtAddr, servers)
	if err != nil {
		t.Errorf("Not expected error `%s` when listing", err.Error())
	}
}

// TODO: test old version case
// TODO: test DB unhealthy case
// TODO: test healthy/unhealthy thresholds
// TODO: test health check timeout case
