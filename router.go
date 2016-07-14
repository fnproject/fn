/*

For keeping a minimum running, perhaps when doing a routing table update, if destination hosts are all
 expired or about to expire we start more.

*/

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/iron-io/go/common"
	"github.com/iron-io/iron_go/cache"
)

var config struct {
	CloudFlare struct {
		Email   string `json:"email"`
		AuthKey string `json:"auth_key"`
	} `json:"cloudflare"`
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

const Version = "0.0.23"

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

	// common.LoadConfigFile(configFile, &config)
	//	common.SetLogging(common.LoggingConfig{To: config.Logging.To, Level: config.Logging.Level, Prefix: config.Logging.Prefix})

	// TODO: validate inputs, iron tokens, cloudflare stuff, etc
	config.CloudFlare.Email = os.Getenv("CLOUDFLARE_EMAIL")
	config.CloudFlare.AuthKey = os.Getenv("CLOUDFLARE_API_KEY")

	log.Println("config:", config)
	log.Infoln("Starting up router version", Version)

	r := mux.NewRouter()

	// dev:
	s := r.PathPrefix("/api").Subrouter()
	// production:
	// s := r.Host("router.irondns.info").Subrouter()
	// s.Handle("/1/projects/{project_id}/register", &Register{})
	s.Handle("/v1/apps", &NewApp{})
	s.HandleFunc("/v1/apps/{app_name}/routes", NewRoute)
	s.HandleFunc("/ping", Ping)
	s.HandleFunc("/version", VersionHandler)
	// s.Handle("/addworker", &WorkerHandler{})
	s.HandleFunc("/", Ping)

	r.HandleFunc("/elb-ping-router", Ping) // for ELB health check

	// for testing app responses, pass in app name, can use localhost
	s4 := r.Queries("app", "").Subrouter()
	// s4.HandleFunc("/appsr", Ping)
	s4.HandleFunc("/{rest:.*}", Run)
	s4.NotFoundHandler = http.HandlerFunc(Run)

	// s3 := r.Queries("rhost", "").Subrouter()
	// s3.HandleFunc("/", ProxyFunc2)

	// This is where all the main incoming traffic goes
	r.NotFoundHandler = http.HandlerFunc(Run)

	http.Handle("/", r)
	port := 8080
	log.Infoln("Router started, listening and serving on port", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf("0.0.4.0:%v", port), nil))
}

type NewApp struct{}

// This registers a new host
func (r *NewApp) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	log.Println("NewApp called!")

	vars := mux.Vars(req)
	projectId := vars["project_id"]
	// token := common.GetToken(req)
	log.Infoln("project_id:", projectId)

	app := App{}
	if !ReadJSON(w, req, &app) {
		return
	}
	log.Infoln("body read into app:", app)
	app.ProjectId = projectId

	_, err := getApp(app.Name)
	if err == nil {
		SendError(w, 400, fmt.Sprintln("An app with this name already exists.", err))
		return
	}

	app.Routes = make(map[string]*Route3)

	// create dns entry
	// TODO: Add project id to this. eg: appname.projectid.iron.computer
	log.Debug("Creating dns entry.")
	regOk := registerHost(w, req, &app)
	if !regOk {
		return
	}

	// todo: do we need to close body?
	err = putApp(&app)
	if err != nil {
		log.Infoln("couldn't create app:", err)
		SendError(w, 400, fmt.Sprintln("Could not create app!", err))
		return
	}
	log.Infoln("registered app:", app)
	v := map[string]interface{}{"app": app}
	SendSuccess(w, "App created successfully.", v)
}

func NewRoute(w http.ResponseWriter, req *http.Request) {
	fmt.Println("NewRoute")
	vars := mux.Vars(req)
	projectId := vars["project_id"]
	appName := vars["app_name"]
	log.Infoln("project_id:", projectId, "app_name", appName)

	route := &Route3{}
	if !ReadJSON(w, req, &route) {
		return
	}
	log.Infoln("body read into route:", route)

	// TODO: validate route

	app, err := getApp(appName)
	if err != nil {
		SendError(w, 400, fmt.Sprintln("This app does not exist. Please create app first.", err))
		return
	}

	if route.Type == "" {
		route.Type = "run"
	}

	// app.Routes = append(app.Routes, route)
	app.Routes[route.Path] = route
	err = putApp(app)
	if err != nil {
		log.Errorln("Couldn't create route!:", err)
		SendError(w, 400, fmt.Sprintln("Could not create route!", err))
		return
	}
	log.Infoln("Route created:", route)
	fmt.Fprintln(w, "Route created successfully.")
}

func getRoute(host string) (*Route, error) {
	log.Infoln("getRoute for host:", host)
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
	log.Infoln("getapp:", name)
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

func VersionHandler(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintln(w, Version)
}
