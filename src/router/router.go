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
	"github.com/iron-io/common"
	"log"
	"math/rand"
	// "net"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"
	"runtime"
	"flag"
	"io/ioutil"
)

var config struct {
	Iron struct {
	Token      string `json:"token"`
	ProjectId  string `json:"project_id"`
} `json:"iron"`
	Logging       struct {
	To     string `json:"to"`
	Level  string `json:"level"`
	Prefix string `json:"prefix"`
}
}

//var routingTable = map[string]*Route{}
var icache = cache.New("routing-table")

func init() {

}

type Route struct {
	// TODO: Change destinations to a simple cache so it can expire entries after 55 minutes (the one we use in common?)
	Host         string `json:"host"`
	Destinations []string  `json:"destinations"`
	ProjectId    string  `json:"project_id"`
	Token        string  `json:"token"` // store this so we can queue up new workers on demand
	CodeName     string  `json:"code_name"`
}

// for adding new hosts
type Route2 struct {
	Host string `json:"host"`
	Dest string `json:"dest"`
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	log.Println("Running on", runtime.NumCPU(), "CPUs")

	var configFile string
	var env string
	flag.StringVar(&configFile, "c", "", "Config file name")
	// when this was e, it was erroring out.
	flag.StringVar(&env, "e2", "development", "environment")

	flag.Parse() // Scans the arg list and sets up flags

	// Deployer is now passing -c in since we're using upstart and it doesn't know what directory to run in
	if configFile == "" {
		configFile = "config_" + env + ".json"
	}

	common.LoadConfig("iron_mq", configFile, &config)
	common.SetLogLevel(config.Logging.Level)
	common.SetLogLocation(config.Logging.To, config.Logging.Prefix)

	icache.Settings.UseConfigMap(map[string]interface{}{"token": config.Iron.Token, "project_id": config.Iron.ProjectId})

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
	//	route := routingTable[host]
	route, err := getRoute(host)
	// choose random dest
	if err != nil {
		common.SendError(w, 400, fmt.Sprintln(w, "Host not configured or error!", err))
		return
	}
	//	if route == nil { // route.Host == "" {
	//		common.SendError(w, 400, fmt.Sprintln(w, "Host not configured!"))
	//		return
	//	}
	destIndex := rand.Intn(len(route.Destinations))
	destUrlString := route.Destinations[destIndex]
	// todo: should check if http:// already exists.
	destUrlString2 := "http://" + destUrlString
	destUrl, err := url.Parse(destUrlString2)
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
			if len(route.Destinations) > 1 {
				fmt.Println("It's a network error, removing this destination from routing table.")
				route.Destinations = append(route.Destinations[:destIndex], route.Destinations[destIndex + 1:]...)
				putRoute(route)
				fmt.Println("New route:", route)
				return
			} else {
				fmt.Println("It's a network error and no other destinations available so we're going to remove it and start new task.")
				route.Destinations = append(route.Destinations[:destIndex], route.Destinations[destIndex + 1:]...)
				putRoute(route)
				fmt.Println("New route:", route)
			}
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
			timeout := time.Second*120
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

	s, err := ioutil.ReadAll(req.Body)
	fmt.Println("req.body:", err, string(s))

	// get project id and token
	projectId := req.FormValue("project_id")
	token := req.FormValue("token")
	codeName := req.FormValue("code_name")
	fmt.Println("project_id:", projectId, "token:", token, "code_name:", codeName)

	// check header for what operation to perform
	routerHeader := req.Header.Get("Iron-Router")
	if routerHeader == "register" {
		route := Route{}
		decoder := json.NewDecoder(req.Body)
		err := decoder.Decode(&route)
		if err != nil {
			common.SendError(w, 400, fmt.Sprintln(w, "Bad json:", err))
		}
		route.ProjectId = projectId
		route.Token = token
		route.CodeName = codeName
		// todo: do we need to close body?
		putRoute(route)
		fmt.Println("registered route:", route)
		fmt.Fprintln(w, "Host registered successfully.")

	} else {
		r2 := Route2{}
		decoder := json.NewDecoder(req.Body)
		err = decoder.Decode(&r2)
		if err != nil {
			common.SendError(w, 400, fmt.Sprintln(w, "Bad json:", err))
		}
		// todo: do we need to close body?
		fmt.Println("DECODED:", r2)
		route, err := getRoute(r2.Host)
		//		route := routingTable[r2.Host]
		if err != nil {
			common.SendError(w, 400, fmt.Sprintln(w, "This host is not registered!", err))
			return
			//			route = &Route{}
		}
		fmt.Println("ROUTE:", route)
		route.Destinations = append(route.Destinations, r2.Dest)
		fmt.Println("ROUTE new:", route)
		putRoute(route)
		//		routingTable[r2.Host] = route
		//		fmt.Println("New routing table:", routingTable)
		fmt.Fprintln(w, "Worker added")
	}
}

func getRoute(host string) (Route, error) {
	rx, err := icache.Get(host)
	route := Route{}
	if err == nil {
		route = rx.(Route)
	}
	return route, err
}

func putRoute(route Route) {
	item := cache.Item{}
	item.Value = route
	icache.Put(route.Host, &item)
}
