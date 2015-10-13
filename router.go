/*

For keeping a minimum running, perhaps when doing a routing table update, if destination hosts are all
 expired or about to expire we start more.

*/

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/iron-io/go/common"
	"github.com/iron-io/golog"
	"github.com/iron-io/iron_go/cache"
	"github.com/iron-io/iron_go/worker"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"reflect"
	"runtime"
	"strings"
	"time"
)

var config struct {
	Cache struct {
		Host      string `json:"host"`
		Token     string `json:"token"`
		ProjectId string `json:"project_id"`
	}
	Iron struct {
		Token      string `json:"token"`
		ProjectId  string `json:"project_id"`
		SuperToken string `json:"super_token"`
		WorkerHost string `json:"worker_host"`
		AuthHost   string `json:"auth_host"`
	} `json:"iron"`
	Logging struct {
		To     string `json:"to"`
		Level  string `json:"level"`
		Prefix string `json:"prefix"`
	}
}

var version = "0.0.19"

//var routingTable = map[string]*Route{}
var icache = cache.New("routing-table")

var (
	ironAuth common.Auther
)

func init() {

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

	common.LoadConfigFile(configFile, &config)
	//	common.SetLogging(common.LoggingConfig{To: config.Logging.To, Level: config.Logging.Level, Prefix: config.Logging.Prefix})

	golog.Infoln("Starting up router version", version)

	runtime.GOMAXPROCS(runtime.NumCPU())
	log.Println("Running on", runtime.NumCPU(), "CPUs")

	//	ironAuth = apiauth.New(config.Iron.AuthHost)

	// cacheConfigMap := map[string]interface{}{
	// 	"token":      config.Cache.Token,
	// 	"project_id": config.Cache.ProjectId,
	// }
	// if config.Cache.Host != "" {
	// 	cacheConfigMap["host"] = config.Cache.Host
	// }
	// icache.Settings.UseConfigMap(cacheConfigMap)

	r := mux.NewRouter()

	// This has to stay above the r.Host() one.
	s2 := r.Headers("Iron-Router", "").Subrouter()
	s2.Handle("/", &WorkerHandler{})

	// dev:
	fmt.Println("Setting /test subrouter")
	s := r.PathPrefix("/test").Subrouter()
	// production:
	// s := r.Host("router.irondns.info").Subrouter()
	s.Handle("/1/projects/{project_id}/register", &Register{})
	s.Handle("/1/projects/{project_id}/apps", &NewApp{})
	s.HandleFunc("/1/projects/{project_id}/apps/{app_name}/routes", NewRoute)
	s.HandleFunc("/ping", Ping)
	s.HandleFunc("/version", Version)
	s.Handle("/addworker", &WorkerHandler{})
	s.HandleFunc("/", Ping)

	r.HandleFunc("/elb-ping-router", Ping) // for ELB health check
	// Now for everyone else:
	//	r.HandleFunc("/", ProxyFunc)

	// for testing app responses, pass in app name
	s4 := r.Queries("app", "").Subrouter()
	s4.HandleFunc("/appsr", Ping)
	s4.HandleFunc("/{rest:.*}", Run)
	s4.NotFoundHandler = http.HandlerFunc(Run)

	s3 := r.Queries("rhost", "").Subrouter()
	s3.HandleFunc("/", ProxyFunc2)
	r.NotFoundHandler = http.HandlerFunc(ProxyFunc)

	http.Handle("/", r)
	port := 8080
	golog.Infoln("Router started, listening and serving on port", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", port), nil))
}

func ProxyFunc2(w http.ResponseWriter, req *http.Request) {
	fmt.Println("proxy2")
	ProxyFunc(w, req)
}

func ProxyFunc(w http.ResponseWriter, req *http.Request) {
	golog.Infoln("HOST:", req.Host)
	host := strings.Split(req.Host, ":")[0]
	rhost := req.FormValue("rhost")
	golog.Infoln("rhost:", rhost)
	if rhost != "" {
		host = rhost
	}

	// We look up the destinations in the routing table and there can be 3 possible scenarios:
	// 1) This host was never registered so we return 404
	// 2) This host has active workers so we do the proxy
	// 3) This host has no active workers so we queue one (or more) up and return a 503 or something with message that says "try again in a minute"
	//	route := routingTable[host]
	golog.Infoln("getting route for host:", host, "--")
	route, err := getRoute(host)
	golog.Infoln("route:", route)
	golog.Infoln("err:", err)
	if err != nil {
		common.SendError(w, 400, fmt.Sprintln("Host not registered or error!", err))
		return
	}
	//	if route == nil { // route.Host == "" {
	//		common.SendError(w, 400, fmt.Sprintln(w, "Host not configured!"))
	//		return
	//	}

	serveEndpoint(w, req, route)
}

func serveEndpoint(w http.ResponseWriter, req *http.Request, route *Route) {
	dlen := len(route.Destinations)
	if dlen == 0 {
		golog.Infoln("No workers running, starting new task.")
		startNewWorker(route)
		common.SendError(w, 500, fmt.Sprintln("No workers running, starting them up..."))
		return
	}
	if dlen < 3 {
		golog.Infoln("Less than three workers running, starting a new task.")
		startNewWorker(route)
	}
	destIndex := rand.Intn(dlen)
	destUrlString := route.Destinations[destIndex]
	// todo: should check if http:// already exists.
	destUrlString2 := "http://" + destUrlString
	destUrl, err := url.Parse(destUrlString2)
	if err != nil {
		removeDestination(route, destIndex, w)
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
		if true || strings.Contains(etype.String(), "net.OpError") { // == reflect.TypeOf(net.OpError{}) { // couldn't figure out a better way to do this
			golog.Infoln("It's a network error so we're going to remove destination.")
			removeDestination(route, destIndex, w)
			serveEndpoint(w, req, route)
			return
		}
		common.SendError(w, 500, fmt.Sprintln("Internal error occurred:", err))
		return
	}
	golog.Infoln("Served!")
}

func removeDestination(route *Route, destIndex int, w http.ResponseWriter) {
	golog.Infoln("Removing destination", destIndex, "from route:", route)
	route.Destinations = append(route.Destinations[:destIndex], route.Destinations[destIndex+1:]...)
	err := putRoute(route)
	if err != nil {
		golog.Infoln("Couldn't update routing table:", err)
		common.SendError(w, 500, fmt.Sprintln("couldn't update routing table", err))
		return
	}
	golog.Infoln("New route after remove destination:", route)
	//	if len(route.Destinations) < 3 {
	//		golog.Infoln("After network error, there are less than three destinations, so starting a new one. ")
	//		// always want at least three running
	//		startNewWorker(route)
	//	}
}

func startNewWorker(route *Route) error {
	golog.Infoln("Starting a new worker")
	// start new worker
	payload := map[string]interface{}{
		"token":      config.Iron.SuperToken,
		"project_id": route.ProjectId,
		"code_name":  route.CodeName,
	}
	if config.Iron.WorkerHost != "" {
		payload["host"] = config.Iron.WorkerHost
	}
	workerapi := worker.New()
	workerapi.Settings.UseConfigMap(payload)
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		golog.Infoln("Couldn't marshal json!", err)
		return err
	}
	timeout := time.Second * time.Duration(1800+rand.Intn(600)) // a little random factor in here to spread out worker deaths
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

type Register struct{}

// This registers a new host
func (r *Register) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	log.Println("Register called!")

	vars := mux.Vars(req)
	projectId := vars["project_id"]
	token := common.GetToken(req)
	golog.Infoln("project_id:", projectId, "token:", token.Token)

	route := Route{}
	if !common.ReadJSON(w, req, &route) {
		return
	}
	golog.Infoln("body read into route:", route)
	route.ProjectId = projectId
	route.Token = token.Token

	_, err := getRoute(route.Host)
	if err == nil {
		common.SendError(w, 400, fmt.Sprintln("This host is already registered. If you believe this is in error, please contact support@iron.io to resolve the issue.", err))
		return
		//			route = &Route{}
	}

	// todo: do we need to close body?
	err = putRoute(&route)
	if err != nil {
		golog.Infoln("couldn't register host:", err)
		common.SendError(w, 400, fmt.Sprintln("Could not register host!", err))
		return
	}
	golog.Infoln("registered route:", route)
	fmt.Fprintln(w, "Host registered successfully.")
}

type NewApp struct{}

// This registers a new host
func (r *NewApp) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	log.Println("NewApp called!")

	vars := mux.Vars(req)
	projectId := vars["project_id"]
	// token := common.GetToken(req)
	golog.Infoln("project_id:", projectId)

	app := App{}
	if !common.ReadJSON(w, req, &app) {
		return
	}
	golog.Infoln("body read into app:", app)
	app.ProjectId = projectId

	_, err := getApp(app.Name)
	if err == nil {
		common.SendError(w, 400, fmt.Sprintln("This app already exists. If you believe this is in error, please contact support@iron.io to resolve the issue.", err))
		return
		//			route = &Route{}
	}

	// todo: do we need to close body?
	err = putApp(&app)
	if err != nil {
		golog.Infoln("couldn't create app:", err)
		common.SendError(w, 400, fmt.Sprintln("Could not create app!", err))
		return
	}
	golog.Infoln("registered app:", app)
	fmt.Fprintln(w, "App created successfully.")
}

func NewRoute(w http.ResponseWriter, req *http.Request) {
	fmt.Println("NewRoute")
	vars := mux.Vars(req)
	projectId := vars["project_id"]
	appName := vars["app_name"]
	golog.Infoln("project_id:", projectId, "app_name", appName)

	route := Route3{}
	if !common.ReadJSON(w, req, &route) {
		return
	}
	golog.Infoln("body read into route:", route)

	// TODO: validate route

	app, err := getApp(appName)
	if err != nil {
		common.SendError(w, 400, fmt.Sprintln("This app does not exist. Please create app first.", err))
		return
	}

	app.Routes = append(app.Routes, route)
	err = putApp(app)
	if err != nil {
		golog.Errorln("Couldn't create route!:", err)
		common.SendError(w, 400, fmt.Sprintln("Could not create route!", err))
		return
	}
	golog.Infoln("Route created:", route)
	fmt.Fprintln(w, "Route created successfully.")
}

type WorkerHandler struct {
}

// When a worker starts up, it calls this
func (wh *WorkerHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	golog.Infoln("AddWorker called!")

	// get project id and token
	projectId := req.FormValue("project_id")
	token := req.FormValue("token")
	//	codeName := req.FormValue("code_name")
	golog.Infoln("project_id:", projectId, "token:", token)

	// check header for what operation to perform
	routerHeader := req.Header.Get("Iron-Router")
	if routerHeader == "register" {

	} else {
		r2 := Route2{}
		if !common.ReadJSON(w, req, &r2) {
			return
		}
		// todo: do we need to close body?
		golog.Infoln("Incoming body from worker:", r2)
		route, err := getRoute(r2.Host)
		if err != nil {
			common.SendError(w, 400, fmt.Sprintln("This host is not registered!", err))
			return
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
		fmt.Fprintln(w, "Worker added")
		golog.Infoln("Worked added.")
	}
}

func getRoute(host string) (*Route, error) {
	golog.Infoln("getRoute for host:", host)
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

func putRoute(route *Route) error {
	item := cache.Item{}
	v, err := json.Marshal(route)
	if err != nil {
		return err
	}
	item.Value = string(v)
	err = icache.Put(route.Host, &item)
	return err
}

func getApp(name string) (*App, error) {
	golog.Infoln("getapp:", name)
	rx, err := icache.Get(name)
	if err != nil {
		return nil, err
	}
	rx2 := []byte(rx.(string))
	app := App{}
	err = json.Unmarshal(rx2, &app)
	if err != nil {
		return nil, err
	}
	return &app, err
}

func putApp(app *App) error {
	item := cache.Item{}
	v, err := json.Marshal(app)
	if err != nil {
		return err
	}
	item.Value = string(v)
	err = icache.Put(app.Name, &item)
	return err
}

func Ping(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintln(w, "pong")
}

func Version(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintln(w, version)
}
