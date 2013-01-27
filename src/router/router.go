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
	"github.com/iron-io/golog"
	"labix.org/v2/mgo"
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
	//	"io/ioutil"
)

var config struct {
	Iron struct {
	Token      string `json:"token"`
	ProjectId  string `json:"project_id"`
} `json:"iron"`
	MongoAuth     common.MongoConfig `json:"mongo_auth"`
	Logging       struct {
	To     string `json:"to"`
	Level  string `json:"level"`
	Prefix string `json:"prefix"`
}
}

var version = "0.0.10"
//var routingTable = map[string]*Route{}
var icache = cache.New("routing-table")

var (
	ironAuth  *common.IronAuth
)

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

	var configFile string
	var env string
	flag.StringVar(&configFile, "c", "", "Config file name")
	// when this was e, it was erroring out.
	flag.StringVar(&env, "e", "development", "environment")

	flag.Parse() // Scans the arg list and sets up flags

	// Deployer is now passing -c in since we're using upstart and it doesn't know what directory to run in
	if configFile == "" {
		configFile = "config_" + env + ".json"
	}

	common.LoadConfig("iron_mq", configFile, &config)
	common.SetLogLevel(config.Logging.Level)
	common.SetLogLocation(config.Logging.To, config.Logging.Prefix)

	golog.Infoln("Starting up router version", version)

	runtime.GOMAXPROCS(runtime.NumCPU())
	log.Println("Running on", runtime.NumCPU(), "CPUs")

	hosts := strings.Join(config.MongoAuth.Hosts, ",")
	session, err := mgo.Dial(hosts)
	if err != nil {
		log.Panicln(err)
	}
	// recompile????
	err = session.DB(config.MongoAuth.Database).Login(config.MongoAuth.Username, config.MongoAuth.Password)
	if err != nil {
		log.Fatalln("Could not log in to db:", err)
	}
	ironAuth = common.NewIronAuth(session, config.MongoAuth.Database)

	icache.Settings.UseConfigMap(map[string]interface{}{"token": config.Iron.Token, "project_id": config.Iron.ProjectId})

	r := mux.NewRouter()

	s := r.Host("router.irondns.info").Subrouter()
	s.Handle("/1/projects/{project_id:[0-9a-fA-F]{24}}/register", &common.AuthHandler{&Register{}, ironAuth})
	s.HandleFunc("/ping", Ping)
	s.Handle("/addworker", &WorkerHandler{})
	s.HandleFunc("/", Ping)

	s2 := s.Headers("Iron-Router", "").Subrouter()
	s2.Handle("/", &WorkerHandler{})

	r.HandleFunc("/ping", Ping) // for ELB health check
	r.HandleFunc("/", ProxyFunc)

	http.Handle("/", r)
	port := 80
	golog.Infoln("Router started, listening and serving on port", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", port), nil))
}

func ProxyFunc(w http.ResponseWriter, req *http.Request) {
	golog.Infoln("HOST:", req.Host)
	host := strings.Split(req.Host, ":")[0]

	// We look up the destinations in the routing table and there can be 3 possible scenarios:
	// 1) This host was never registered so we return 404
	// 2) This host has active workers so we do the proxy
	// 3) This host has no active workers so we queue one (or more) up and return a 503 or something with message that says "try again in a minute"
	//	route := routingTable[host]
	golog.Infoln("getting route for host:", host)
	route, err := getRoute(host)
	// choose random dest
	if err != nil {
		common.SendError(w, 400, fmt.Sprintln("Host not registered or error!", err))
		return
	}
	//	if route == nil { // route.Host == "" {
	//		common.SendError(w, 400, fmt.Sprintln(w, "Host not configured!"))
	//		return
	//	}
	dlen := len(route.Destinations)
	if dlen == 0 {
		golog.Infoln("No workers running, starting new task.")
		startNewWorker(route)
		common.SendError(w, 500, fmt.Sprintln("No workers running, starting them up..."))
		return
	}
	if dlen < 3 {
		golog.Infoln("Only one worker running, starting a new task.")
		startNewWorker(route)
	}
	serveEndpoint(w, req, route)
}

func serveEndpoint(w http.ResponseWriter, req *http.Request, route *Route) {
	dlen := len(route.Destinations)
	destIndex := rand.Intn(dlen)
	destUrlString := route.Destinations[destIndex]
	// todo: should check if http:// already exists.
	destUrlString2 := "http://" + destUrlString
	destUrl, err := url.Parse(destUrlString2)
	if err != nil {
		// todo: should remove destination here
		golog.Infoln("error!", err)
		common.SendError(w, 500, fmt.Sprintln("Internal error occurred:", err))
		return
	}
	// todo: check destination runtime and remove it if it's expired so we don't send requests to an endpoint that is about to be killed
	golog.Infoln("proxying to", destUrl)
	proxy := NewSingleHostReverseProxy(destUrl)
	err = proxy.ServeHTTP(w, req)
	if err != nil {
		golog.Infoln("Error proxying!", err)
		etype := reflect.TypeOf(err)
		golog.Infoln("err type:", etype)
		// can't figure out how to compare types so comparing strings.... lame.
		if strings.Contains(etype.String(), "net.OpError") { // == reflect.TypeOf(net.OpError{}) { // couldn't figure out a better way to do this
			golog.Infoln("It's a network error so we're going to remove destination.")
			route.Destinations = append(route.Destinations[:destIndex], route.Destinations[destIndex + 1:]...)
			err := putRoute(route)
			if err != nil {
				golog.Infoln("Couldn't update routing table:", err)
				common.SendError(w, 500, fmt.Sprintln("couldn't update routing table", err))
				return
			}
			golog.Infoln("New route:", route)
			if len(route.Destinations) < 3 {
				golog.Infoln("After network error, there are less than three destinations, so starting a new one. ")
				// always want at least three running
				startNewWorker(route)
			}
			serveEndpoint(w, req, route)
			return
		}
		common.SendError(w, 500, fmt.Sprintln("Internal error occurred:", err))
		return
	}
	golog.Infoln("Served!")
}

func startNewWorker(route *Route) (error) {
	golog.Infoln("Starting a new worker")
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
		golog.Infoln("Couldn't marshal json!", err)
		return err
	}
	timeout := time.Second*time.Duration(1800 + rand.Intn(600)) // a little random factor in here to spread out worker deaths
	task := worker.Task{
		CodeName: route.CodeName,
		Payload:  string(jsonPayload),
		Timeout:  &timeout, // let's have these die quickly while testing
	}
	tasks := make([]worker.Task, 1)
	tasks[0] = task
	taskIds, err := workerapi.TaskQueue(tasks...)
	golog.Infoln("Tasks queued.", taskIds)
	if err != nil {
		golog.Infoln("Couldn't queue up worker!", err)
		return err
	}
	return err
}

type Register struct {}

// This registers a new host
func (r *Register) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	log.Println("Register called!")

	//	s, err := ioutil.ReadAll(req.Body)
	//	fmt.Println("req.body:", err, string(s))

	// get project id and token
	vars := mux.Vars(req)
	projectId := vars["project_id"]
	//	projectId := req.FormValue("project_id")
	//	token := req.FormValue("token")
	//	codeName := req.FormValue("code_name")
	token := ironAuth.GetToken(req)
	golog.Infoln("project_id:", projectId, "token:", token.Token)

	route := Route{}
	if !common.ReadJSON(w, req, &route) {
		return
	}
	golog.Infoln("body read into route:", route)
	route.ProjectId = projectId
	route.Token = token.Token
	// todo: do we need to close body?
	err := putRoute(&route)
	if err != nil {
		golog.Infoln("couldn't register host:", err)
		common.SendError(w, 400, fmt.Sprintln("Could not register host!", err))
		return
	}
	err = putRoute(&route)
	if err != nil {
		golog.Infoln("couldn't register host:", err)
		common.SendError(w, 400, fmt.Sprintln("Could not register host!", err))
		return
	}
	golog.Infoln("registered route:", route)
	fmt.Fprintln(w, "Host registered successfully.")
}

type WorkerHandler struct {
}

// When a worker starts up, it calls this
func (wh *WorkerHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	golog.Infoln("AddWorker called!")

	// get project id and token
	projectId := req.FormValue("project_id")
	token := req.FormValue("token")
	codeName := req.FormValue("code_name")
	golog.Infoln("project_id:", projectId, "token:", token, "code_name:", codeName)

	// check header for what operation to perform
	routerHeader := req.Header.Get("Iron-Router")
	if routerHeader == "register" {

	} else {
		r2 := Route2{}
		if !common.ReadJSON(w, req, &r2) {
			return
		}
		// todo: do we need to close body?
		golog.Infoln("DECODED:", r2)
		route, err := getRoute(r2.Host)
		//		route := routingTable[r2.Host]
		if err != nil {
			common.SendError(w, 400, fmt.Sprintln("This host is not registered!", err))
			return
			//			route = &Route{}
		}
		golog.Infoln("ROUTE:", route)
		route.Destinations = append(route.Destinations, r2.Dest)
		golog.Infoln("ROUTE new:", route)
		err = putRoute(route)
		if err != nil {
			golog.Infoln("couldn't register host:", err)
			common.SendError(w, 400, fmt.Sprintln("Could not register host!", err))
			return
		}
		//		routingTable[r2.Host] = route
		//		fmt.Println("New routing table:", routingTable)
		fmt.Fprintln(w, "Worker added")
	}
}

func getRoute(host string) (*Route, error) {
	rx, err := icache.Get(host)
	if err != nil {
		return nil, err
	}
	rx2 := []byte(rx.(string))
	route := Route{}
	err = json.Unmarshal(rx2, &route)
	if err != nil {
		return nil, err
	}
	return &route, err
}

func putRoute(route *Route) (error) {
	item := cache.Item{}
	v, err := json.Marshal(route)
	if err != nil {
		return err
	}
	item.Value = string(v)
	err = icache.Put(route.Host, &item)
	return err
}

func Ping(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintln(w, "pong")
}
