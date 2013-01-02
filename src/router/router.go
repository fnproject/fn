/*

For keeping a minimum running, perhaps when doing a routing table update, if destination hosts are all
 expired or about to expire we start more. 

*/

package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/iron-io/iron_go/cache"
	"github.com/iron-io/iron_go/worker"
	"log"
	"math/rand"
	// "net"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"
)

var routingTable = map[string]Route{}
var icache = cache.New("routertest")

func init() {
	icache.Settings.UseConfigMap(map[string]interface{}{"token": "MWx0VfngzsCu0W8NAYw7S2lNrgo", "project_id": "50e227be8e7d14359b001373"})
}

type Route struct {
	// TODO: Change destinations to a simple cache so it can expire entries after 55 minutes (the one we use in common?)
	Destinations []string
	ProjectId    string
	Token        string // store this so we can queue up new workers on demand
	CodeName     string
}

// for adding new hosts
type Route2 struct {
	Host string `json:"host"`
	Dest string `json:"dest"`
}

func main() {
	// verbose := flag.Bool("v", true, "should every proxy request be logged to stdout")
	// flag.Parse()

	r := mux.NewRouter()
	s := r.Headers("Iron-Router", "").Subrouter()
	s.HandleFunc("/", AddWorker)
	r.HandleFunc("/addworker", AddWorker)

	r.HandleFunc("/", ProxyFunc)

	http.Handle("/", r)
	port := 80
	fmt.Println("listening and serving on port", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", port), nil))
}

func ProxyFunc(w http.ResponseWriter, req *http.Request) {
	fmt.Println("HOST:", req.Host)
	host := strings.Split(req.Host, ":")[0]

	// We look up the destinations in the routing table and there can be 3 possible scenarios:
	// 1) This host was never registered so we return 404
	// 2) This host has active workers so we do the proxy
	// 3) This host has no active workers so we queue one (or more) up and return a 503 or something with message that says "try again in a minute"
	route := routingTable[host]
	// choose random dest
	if len(route.Destinations) == 0 {
		fmt.Fprintln(w, "No matching routes!")
		return
	}
	destUrls := route.Destinations[rand.Intn(len(route.Destinations))]
	// todo: should check if http:// already exists.
	destUrls = "http://" + destUrls
	destUrl, err := url.Parse(destUrls)
	if err != nil {
		fmt.Println("error!", err)
		panic(err)
	}
	fmt.Println("proxying to", destUrl)
	proxy := NewSingleHostReverseProxy(destUrl)
	err = proxy.ServeHTTP(w, req)
	if err != nil {
		fmt.Println("Error proxying!", err)
		etype := reflect.TypeOf(err)
		fmt.Println("err type:", etype)
		w.WriteHeader(http.StatusInternalServerError)
		// can't figure out how to compare types so comparing strings.... lame. 
		if strings.Contains(etype.String(), "net.OpError") { // == reflect.TypeOf(net.OpError{}) { // couldn't figure out a better way to do this
			fmt.Println("It's a network error, so we're going to start new task.")
			// start new worker
			payload := map[string]interface{}{
				"token":      route.Token,
				"project_id": route.ProjectId,
				"code_name":  route.CodeName,
			}
			workerapi := worker.New()
			workerapi.Settings.UseConfigMap(payload)
			jsonPayload, err := json.Marshal(payload)
			if err != nil {
				fmt.Println("Couldn't marshal json!", err)
				return
			}
			timeout := time.Second * 300
			task := worker.Task{
				CodeName: route.CodeName,
				Payload:  string(jsonPayload),
				Timeout:  &timeout, // let's have these die quickly while testing
			}
			tasks := make([]worker.Task, 1)
			tasks[0] = task
			taskIds, err := workerapi.TaskQueue(tasks...)
			fmt.Println("Tasks queued.", taskIds)
			if err != nil {
				fmt.Println("Couldn't queue up worker!", err)
				return
			}

		}
		// start new worker if it's a connection error
		return
	}
	fmt.Println("Served!")
	// todo: how to handle destination failures. I got this in log output when testing a bad endpoint:
	// 2012/12/26 23:22:08 http: proxy error: dial tcp 127.0.0.1:8082: connection refused
}

// When a worker starts up, it calls this
func AddWorker(w http.ResponseWriter, req *http.Request) {
	log.Println("AddWorker called!")

	r2 := Route2{}
	decoder := json.NewDecoder(req.Body)
	decoder.Decode(&r2)
	// todo: do we need to close body?
	fmt.Println("DECODED:", r2)

	// get project id and token
	projectId := req.FormValue("project_id")
	token := req.FormValue("token")
	codeName := req.FormValue("code_name")
	fmt.Println("project_id:", projectId, "token:", token, "code_name:", codeName)

	// todo: routing table should be in mongo (or IronCache?) so all routers can update/read from it.
	// todo: one cache entry per host domain
	route := routingTable[r2.Host]
	fmt.Println("ROUTE:", route)
	route.Destinations = append(route.Destinations, r2.Dest)
	route.ProjectId = projectId
	route.Token = token
	route.CodeName = codeName
	fmt.Println("ROUTE:", route)
	routingTable[r2.Host] = route
	fmt.Println("New routing table:", routingTable)
	fmt.Fprintln(w, "Worker added")
}
