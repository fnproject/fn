package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/runner"
	"github.com/fnproject/fn/api/runner/common"
	"github.com/fnproject/fn/api/runner/task"
	"github.com/gin-gonic/gin"
	"github.com/go-openapi/strfmt"
	cache "github.com/patrickmn/go-cache"
)

type runnerResponse struct {
	RequestID string            `json:"request_id,omitempty"`
	Error     *models.ErrorBody `json:"error,omitempty"`
}

func toEnvName(envtype, name string) string {
	name = strings.ToUpper(strings.Replace(name, "-", "_", -1))
	if envtype == "" {
		return name
	}
	return fmt.Sprintf("%s_%s", envtype, name)
}

func (s *Server) handleRequest(c *gin.Context, enqueue models.Enqueue) {
	if strings.HasPrefix(c.Request.URL.Path, "/v1") {
		c.Status(http.StatusNotFound)
		return
	}

	ctx := c.Request.Context()

	reqID := id.New().String()
	ctx, log := common.LoggerWithFields(ctx, logrus.Fields{"call_id": reqID})

	var err error
	var payload io.Reader

	if c.Request.Method == "POST" {
		payload = c.Request.Body
		// Load complete body and close
		defer func() {
			io.Copy(ioutil.Discard, c.Request.Body)
			c.Request.Body.Close()
		}()
	} else if c.Request.Method == "GET" {
		reqPayload := c.Request.URL.Query().Get("payload")
		payload = strings.NewReader(reqPayload)
	}

	r, routeExists := c.Get(api.Path)
	if !routeExists {
		r = "/"
	}

	reqRoute := &models.Route{
		AppName: c.MustGet(api.AppName).(string),
		Path:    path.Clean(r.(string)),
	}

	s.FireBeforeDispatch(ctx, reqRoute)

	appName := reqRoute.AppName
	path := reqRoute.Path

	app, err := s.Datastore.GetApp(ctx, appName)
	if err != nil {
		handleErrorResponse(c, err)
		return
	} else if app == nil {
		handleErrorResponse(c, models.ErrAppsNotFound)
		return
	}

	log.WithFields(logrus.Fields{"app": appName, "path": path}).Debug("Finding route on datastore")
	route, err := s.loadroute(ctx, appName, path)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	if route == nil {
		handleErrorResponse(c, models.ErrRoutesNotFound)
		return
	}

	log = log.WithFields(logrus.Fields{"app": appName, "path": route.Path, "image": route.Image})
	log.Debug("Got route from datastore")

	if s.serve(ctx, c, appName, route, app, path, reqID, payload, enqueue) {
		s.FireAfterDispatch(ctx, reqRoute)
		return
	}

	handleErrorResponse(c, models.ErrRoutesNotFound)
}

func (s *Server) loadroute(ctx context.Context, appName, path string) (*models.Route, error) {
	if route, ok := s.cacheget(appName, path); ok {
		return route, nil
	}
	key := routeCacheKey(appName, path)
	resp, err := s.singleflight.do(
		key,
		func() (interface{}, error) {
			return s.Datastore.GetRoute(ctx, appName, path)
		},
	)
	if err != nil {
		return nil, err
	}
	route := resp.(*models.Route)
	s.routeCache.Set(key, route, cache.DefaultExpiration)
	return route, nil
}

// TODO: Should remove *gin.Context from these functions, should use only context.Context
func (s *Server) serve(ctx context.Context, c *gin.Context, appName string, route *models.Route, app *models.App, path, reqID string, payload io.Reader, enqueue models.Enqueue) (ok bool) {
	ctx, log := common.LoggerWithFields(ctx, logrus.Fields{"app": appName, "route": route.Path, "image": route.Image})

	params, match := matchRoute(route.Path, path)
	if !match {
		return false
	}

	var stdout bytes.Buffer // TODO: should limit the size of this, error if gets too big. akin to: https://golang.org/pkg/io/#LimitReader

	if route.Format == "" {
		route.Format = "default"
	}

	// baseVars are the vars on the route & app, not on this specific request [for hot functions]
	baseVars := make(map[string]string, len(app.Config)+len(route.Config)+3)
	baseVars["FN_FORMAT"] = route.Format
	baseVars["APP_NAME"] = appName
	baseVars["ROUTE"] = route.Path
	// TODO add this back after #193 #195 (fix async RAM)
	// baseVars["MEMORY_MB"] = fmt.Sprintf("%d", route.Memory)

	// app config
	for k, v := range app.Config {
		k = toEnvName("", k)
		baseVars[k] = v
	}
	for k, v := range route.Config {
		k = toEnvName("", k)
		baseVars[k] = v
	}

	// envVars contains the full set of env vars, per request + base
	envVars := make(map[string]string, len(baseVars)+len(params)+len(c.Request.Header)+3)

	for k, v := range baseVars {
		envVars[k] = v
	}

	envVars["CALL_ID"] = reqID
	envVars["METHOD"] = c.Request.Method
	envVars["REQUEST_URL"] = fmt.Sprintf("%v//%v%v", func() string {
		if c.Request.TLS == nil {
			return "http"
		}
		return "https"
	}(), c.Request.Host, c.Request.URL.String())

	// params
	for _, param := range params {
		envVars[toEnvName("PARAM", param.Key)] = param.Value
	}

	// headers
	for header, value := range c.Request.Header {
		envVars[toEnvName("HEADER", header)] = strings.Join(value, ", ")
	}

	cfg := &task.Config{
		AppName:      appName,
		Path:         route.Path,
		BaseEnv:      baseVars,
		Env:          envVars,
		Format:       route.Format,
		ID:           reqID,
		Image:        route.Image,
		Memory:       route.Memory,
		Stdin:        payload,
		Stdout:       &stdout,
		Timeout:      time.Duration(route.Timeout) * time.Second,
		IdleTimeout:  time.Duration(route.IdleTimeout) * time.Second,
		ReceivedTime: time.Now(),
		Ready:        make(chan struct{}),
	}

	// ensure valid values
	if cfg.Timeout <= 0 {
		cfg.Timeout = runner.DefaultTimeout
	}
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = runner.DefaultIdleTimeout
	}

	s.Runner.Enqueue()
	createdAt := strfmt.DateTime(time.Now())
	newTask := &models.Task{}
	newTask.Image = &cfg.Image
	newTask.ID = cfg.ID
	newTask.CreatedAt = createdAt
	newTask.Path = route.Path
	newTask.EnvVars = cfg.Env
	newTask.AppName = cfg.AppName

	switch route.Type {
	case "async":
		// Read payload
		pl, err := ioutil.ReadAll(cfg.Stdin)
		if err != nil {
			handleErrorResponse(c, models.ErrInvalidPayload)
			return true
		}
		// Create Task
		priority := int32(0)
		newTask.Priority = &priority
		newTask.Payload = string(pl)

		// Push to queue
		_, err = enqueue(c, s.MQ, newTask)
		if err != nil {
			handleErrorResponse(c, err)
			return true
		}

		log.Info("Added new task to queue")
		c.JSON(http.StatusAccepted, map[string]string{"call_id": newTask.ID})

	default:
		result, err := s.Runner.RunTrackedTask(newTask, ctx, cfg)
		if result != nil {
			waitTime := result.StartTime().Sub(cfg.ReceivedTime)
			c.Header("XXX-FXLB-WAIT", waitTime.String())
		}

		if err != nil {
			c.JSON(http.StatusInternalServerError, runnerResponse{
				RequestID: cfg.ID,
				Error: &models.ErrorBody{
					Message: err.Error(),
				},
			})
			log.WithError(err).Error("Failed to run task")
			break
		}

		for k, v := range route.Headers {
			c.Header(k, v[0])
		}

		// this will help users to track sync execution in a manner of async
		// FN_CALL_ID is an equivalent of call_id
		c.Header("FN_CALL_ID", newTask.ID)

		switch result.Status() {
		case "success":
			c.Data(http.StatusOK, "", stdout.Bytes())
		case "timeout":
			c.JSON(http.StatusGatewayTimeout, runnerResponse{
				RequestID: cfg.ID,
				Error: &models.ErrorBody{
					Message: models.ErrRunnerTimeout.Error(),
				},
			})
		default:
			c.JSON(http.StatusInternalServerError, runnerResponse{
				RequestID: cfg.ID,
				Error: &models.ErrorBody{
					Message: result.Error(),
				},
			})
		}
	}

	return true
}

var fakeHandler = func(http.ResponseWriter, *http.Request, Params) {}

func matchRoute(baseRoute, route string) (Params, bool) {
	tree := &node{}
	tree.addRoute(baseRoute, fakeHandler)
	handler, p, _ := tree.getValue(route)
	if handler == nil {
		return nil, false
	}

	return p, true
}
