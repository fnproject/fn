package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/iron-io/iron_go/cache"
)

var icache = cache.New("routing-table")
var config *Config

func New(conf *Config) *Api {
	config = conf
	api := &Api{}
	return api
}

type Api struct {
}

func (api *Api) Start() {

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
	log.Fatal(http.ListenAndServe(fmt.Sprintf("0.0.0.0:%v", port), nil))
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
	v := map[string]interface{}{"app": app, "msg": "App created successfully."}
	SendSuccess(w, "App created successfully.", v)
}

func NewRoute(w http.ResponseWriter, req *http.Request) {
	fmt.Println("NewRoute")
	vars := mux.Vars(req)
	projectId := vars["project_id"]
	appName := vars["app_name"]
	log.Infoln("project_id: ", projectId, "app: ", appName)

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
	v := map[string]interface{}{"url": fmt.Sprintf("http://%v%v", app.Dns, route.Path), "msg": "Route created successfully."}
	SendSuccess(w, "Route created successfully.", v)
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
