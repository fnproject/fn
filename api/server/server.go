package server

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"math"
	"net/http"
	"path"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/ifaces"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/functions/api/runner"
	"github.com/iron-io/functions/api/server/internal/routecache"
	"github.com/iron-io/runner/common"
)

// Would be nice to not have this is a global, but hard to pass things around to the
// handlers in Gin without it.
var Api *Server

type Server struct {
	Runner          *runner.Runner
	Router          *gin.Engine
	MQ              models.MessageQueue
	AppListeners    []ifaces.AppListener
	SpecialHandlers []ifaces.SpecialHandler
	Enqueue         models.Enqueue

	tasks chan runner.TaskRequest

	mu        sync.Mutex // protects hotroutes
	hotroutes map[string]*routecache.Cache

	singleflight singleflight // singleflight assists Datastore
	Datastore    models.Datastore
}

func New(ctx context.Context, ds models.Datastore, mq models.MessageQueue, r *runner.Runner, tasks chan runner.TaskRequest, enqueue models.Enqueue) *Server {
	Api = &Server{
		Runner:    r,
		Router:    gin.New(),
		Datastore: ds,
		MQ:        mq,
		hotroutes: make(map[string]*routecache.Cache),
		tasks:     tasks,
		Enqueue:   enqueue,
	}

	Api.Router.Use(func(c *gin.Context) {
		ctx, _ := common.LoggerWithFields(ctx, extractFields(c))
		c.Set("ctx", ctx)
		c.Next()
	})
	Api.primeCache()

	return Api
}

func (s *Server) primeCache() {
	logrus.Info("priming cache with known routes")
	apps, err := s.Datastore.GetApps(nil)
	if err != nil {
		logrus.WithError(err).Error("cannot prime cache - could not load application list")
		return
	}
	for _, app := range apps {
		routes, err := s.Datastore.GetRoutesByApp(app.Name, &models.RouteFilter{AppName: app.Name})
		if err != nil {
			logrus.WithError(err).WithField("appName", app.Name).Error("cannot prime cache - could not load routes")
			continue
		}

		entries := len(routes)
		// The idea here is to prevent both extremes: cache being too small that is ineffective,
		// or too large that it takes too much memory. Up to 1k routes, the cache will try to hold
		// all routes in the memory, thus taking up to 48K per application. After this threshold,
		// it will keep 1024 routes + 20% of the total entries - in a hybrid incarnation of Pareto rule
		// 1024+20% of the remaining routes will likelly be responsible for 80% of the workload.
		if entries > cacheParetoThreshold {
			entries = int(math.Ceil(float64(entries-1024)*0.2)) + 1024
		}
		s.hotroutes[app.Name] = routecache.New(entries)

		for i := 0; i < entries; i++ {
			s.refreshcache(app.Name, routes[i])
		}
	}
	logrus.Info("cached prime")
}

// AddAppListener adds a listener that will be notified on App changes.
func (s *Server) AddAppListener(listener ifaces.AppListener) {
	s.AppListeners = append(s.AppListeners, listener)
}

func (s *Server) FireBeforeAppUpdate(ctx context.Context, app *models.App) error {
	for _, l := range s.AppListeners {
		err := l.BeforeAppUpdate(ctx, app)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) FireAfterAppUpdate(ctx context.Context, app *models.App) error {
	for _, l := range s.AppListeners {
		err := l.AfterAppUpdate(ctx, app)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) AddSpecialHandler(handler ifaces.SpecialHandler) {
	s.SpecialHandlers = append(s.SpecialHandlers, handler)
}

func (s *Server) UseSpecialHandlers(ginC *gin.Context) error {
	c := &SpecialHandlerContext{
		server:     s,
		ginContext: ginC,
	}
	for _, l := range s.SpecialHandlers {
		err := l.Handle(c)
		if err != nil {
			return err
		}
	}
	// now call the normal runner call
	s.handleRequest(ginC, nil)
	return nil
}

func DefaultEnqueue(ctx context.Context, mq models.MessageQueue, task *models.Task) (*models.Task, error) {
	ctx, _ = common.LoggerWithFields(ctx, logrus.Fields{"call_id": task.ID})
	return mq.Push(ctx, task)
}

func (s *Server) handleRunnerRequest(c *gin.Context) {
	s.handleRequest(c, s.Enqueue)
}

// cacheParetoThreshold is both the mark from which the LRU starts caching only
// the most likely hot routes, and also as a stopping mark for the cache priming
// during start.
const cacheParetoThreshold = 1024

func (s *Server) cacheget(appname, path string) (*models.Route, bool) {
	s.mu.Lock()
	cache, ok := s.hotroutes[appname]
	if !ok {
		s.mu.Unlock()
		return nil, false
	}
	route, ok := cache.Get(path)
	s.mu.Unlock()
	return route, ok
}

func (s *Server) refreshcache(appname string, route *models.Route) {
	s.mu.Lock()
	cache := s.hotroutes[appname]
	cache.Refresh(route)
	s.mu.Unlock()
}

func (s *Server) resetcache(appname string, delta int) {
	s.mu.Lock()
	hr, ok := s.hotroutes[appname]
	if !ok {
		s.hotroutes[appname] = routecache.New(0)
		hr = s.hotroutes[appname]
	}
	s.hotroutes[appname] = routecache.New(hr.MaxEntries + delta)
	s.mu.Unlock()
}

func (s *Server) handleTaskRequest(c *gin.Context) {
	ctx, _ := common.LoggerWithFields(c, nil)
	switch c.Request.Method {
	case "GET":
		task, err := s.MQ.Reserve(ctx)
		if err != nil {
			logrus.WithError(err).Error()
			c.JSON(http.StatusInternalServerError, simpleError(models.ErrRoutesList))
			return
		}
		c.JSON(http.StatusAccepted, task)
	case "DELETE":
		body, err := ioutil.ReadAll(c.Request.Body)
		if err != nil {
			logrus.WithError(err).Error()
			c.JSON(http.StatusInternalServerError, err)
			return
		}
		var task models.Task
		if err = json.Unmarshal(body, &task); err != nil {
			logrus.WithError(err).Error()
			c.JSON(http.StatusInternalServerError, err)
			return
		}

		if err := s.MQ.Delete(ctx, &task); err != nil {
			logrus.WithError(err).Error()
			c.JSON(http.StatusInternalServerError, err)
			return
		}
		c.JSON(http.StatusAccepted, task)
	}
}

func extractFields(c *gin.Context) logrus.Fields {
	fields := logrus.Fields{"action": path.Base(c.HandlerName())}
	for _, param := range c.Params {
		fields[param.Key] = param.Value
	}
	return fields
}

func (s *Server) Run() {
	s.bindHandlers()

	// By default it serves on :8080 unless a
	// PORT environment variable was defined.
	go s.Router.Run()
}

func (s *Server) bindHandlers() {
	engine := s.Router

	engine.GET("/", handlePing)
	engine.GET("/version", handleVersion)

	v1 := engine.Group("/v1")
	{
		v1.GET("/apps", handleAppList)
		v1.POST("/apps", s.handleAppCreate)

		v1.GET("/apps/:app", handleAppGet)
		v1.PUT("/apps/:app", handleAppUpdate)
		v1.DELETE("/apps/:app", handleAppDelete)

		v1.GET("/routes", handleRouteList)

		apps := v1.Group("/apps/:app")
		{
			apps.GET("/routes", handleRouteList)
			apps.POST("/routes", s.handleRouteCreate)
			apps.GET("/routes/*route", handleRouteGet)
			apps.PUT("/routes/*route", handleRouteUpdate)
			apps.DELETE("/routes/*route", s.handleRouteDelete)
		}
	}

	engine.DELETE("/tasks", s.handleTaskRequest)
	engine.GET("/tasks", s.handleTaskRequest)
	engine.Any("/r/:app/*route", s.handleRunnerRequest)

	// This final route is used for extensions, see Server.Add
	engine.NoRoute(handleSpecial)
}

var ErrInternalServerError = errors.New("Something unexpected happened on the server")

func simpleError(err error) *models.Error {
	return &models.Error{&models.ErrorBody{Message: err.Error()}}
}

type appResponse struct {
	Message string      `json:"message"`
	App     *models.App `json:"app"`
}

type appsResponse struct {
	Message string      `json:"message"`
	Apps    models.Apps `json:"apps"`
}

type routeResponse struct {
	Message string        `json:"message"`
	Route   *models.Route `json:"route"`
}

type routesResponse struct {
	Message string        `json:"message"`
	Routes  models.Routes `json:"routes"`
}

type tasksResponse struct {
	Message string      `json:"message"`
	Task    models.Task `json:"tasksResponse"`
}
