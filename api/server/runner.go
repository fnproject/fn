package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/go-openapi/strfmt"
	"gitlab-odx.oracle.com/odx/functions/api"
	"gitlab-odx.oracle.com/odx/functions/api/id"
	"gitlab-odx.oracle.com/odx/functions/api/models"
	"gitlab-odx.oracle.com/odx/functions/api/runner"
	"gitlab-odx.oracle.com/odx/functions/api/runner/common"
	"gitlab-odx.oracle.com/odx/functions/api/runner/task"
)

type runnerResponse struct {
	RequestID string            `json:"request_id,omitempty"`
	Error     *models.ErrorBody `json:"error,omitempty"`
}

func (s *Server) handleSpecial(c *gin.Context) {
	ctx := c.MustGet("ctx").(context.Context)
	log := common.Logger(ctx)

	ctx = context.WithValue(ctx, api.AppName, "")
	c.Set(api.AppName, "")
	ctx = context.WithValue(ctx, api.Path, c.Request.URL.Path)
	c.Set(api.Path, c.Request.URL.Path)

	ctx, err := s.UseSpecialHandlers(ctx, c.Request, c.Writer)
	if err == ErrNoSpecialHandlerFound {
		log.WithError(err).Errorln("Not special handler found")
		c.JSON(http.StatusNotFound, http.StatusText(http.StatusNotFound))
		return
	} else if err != nil {
		log.WithError(err).Errorln("Error using special handler!")
		c.JSON(http.StatusInternalServerError, simpleError(errors.New("Failed to run function")))
		return
	}

	c.Set("ctx", ctx)
	c.Set(api.AppName, ctx.Value(api.AppName).(string))
	if c.MustGet(api.AppName).(string) == "" {
		log.WithError(err).Errorln("Specialhandler returned empty app name")
		c.JSON(http.StatusBadRequest, simpleError(models.ErrRunnerRouteNotFound))
		return
	}

	// now call the normal runner call
	s.handleRequest(c, nil)
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

	ctx := c.MustGet("ctx").(context.Context)

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
	if err != nil || app == nil {
		log.WithError(err).Error(models.ErrAppsNotFound)
		c.JSON(http.StatusNotFound, simpleError(models.ErrAppsNotFound))
		return
	}

	log.WithFields(logrus.Fields{"app": appName, "path": path}).Debug("Finding route on datastore")
	routes, err := s.loadroutes(ctx, models.RouteFilter{AppName: appName, Path: path})
	if err != nil {
		log.WithError(err).Error(models.ErrRoutesList)
		c.JSON(http.StatusInternalServerError, simpleError(models.ErrRoutesList))
		return
	}

	if len(routes) == 0 {
		log.WithError(err).Error(models.ErrRunnerRouteNotFound)
		c.JSON(http.StatusNotFound, simpleError(models.ErrRunnerRouteNotFound))
		return
	}

	log.WithField("routes", len(routes)).Debug("Got routes from datastore")
	route := routes[0]
	log = log.WithFields(logrus.Fields{"app": appName, "path": route.Path, "image": route.Image})

	if s.serve(ctx, c, appName, route, app, path, reqID, payload, enqueue) {
		s.FireAfterDispatch(ctx, reqRoute)
		return
	}

	log.Error(models.ErrRunnerRouteNotFound)
	c.JSON(http.StatusNotFound, simpleError(models.ErrRunnerRouteNotFound))
}

func (s *Server) loadroutes(ctx context.Context, filter models.RouteFilter) ([]*models.Route, error) {
	if route, ok := s.cacheget(filter.AppName, filter.Path); ok {
		return []*models.Route{route}, nil
	}
	resp, err := s.singleflight.do(
		filter,
		func() (interface{}, error) {
			return s.Datastore.GetRoutesByApp(ctx, filter.AppName, &filter)
		},
	)
	return resp.([]*models.Route), err
}

// TODO: Should remove *gin.Context from these functions, should use only context.Context
func (s *Server) serve(ctx context.Context, c *gin.Context, appName string, found *models.Route, app *models.App, route, reqID string, payload io.Reader, enqueue models.Enqueue) (ok bool) {
	ctx, log := common.LoggerWithFields(ctx, logrus.Fields{"app": appName, "route": found.Path, "image": found.Image})

	params, match := matchRoute(found.Path, route)
	if !match {
		return false
	}

	var stdout bytes.Buffer // TODO: should limit the size of this, error if gets too big. akin to: https://golang.org/pkg/io/#LimitReader

	envVars := map[string]string{
		"METHOD":   c.Request.Method,
		"APP_NAME": appName,
		"ROUTE":    found.Path,
		"REQUEST_URL": fmt.Sprintf("%v//%v%v", func() string {
			if c.Request.TLS == nil {
				return "http"
			}
			return "https"
		}(), c.Request.Host, c.Request.URL.String()),
		"CALL_ID": reqID,
	}

	// app config
	for k, v := range app.Config {
		envVars[toEnvName("", k)] = v
	}
	for k, v := range found.Config {
		envVars[toEnvName("", k)] = v
	}

	// params
	for _, param := range params {
		envVars[toEnvName("PARAM", param.Key)] = param.Value
	}

	// headers
	for header, value := range c.Request.Header {
		envVars[toEnvName("HEADER", header)] = strings.Join(value, " ")
	}

	cfg := &task.Config{
		AppName:      appName,
		Path:         found.Path,
		Env:          envVars,
		Format:       found.Format,
		ID:           reqID,
		Image:        found.Image,
		Memory:       found.Memory,
		Stdin:        payload,
		Stdout:       &stdout,
		Timeout:      time.Duration(found.Timeout) * time.Second,
		IdleTimeout:  time.Duration(found.IdleTimeout) * time.Second,
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
	newTask.Path = found.Path
	newTask.EnvVars = cfg.Env
	newTask.AppName = cfg.AppName

	switch found.Type {
	case "async":
		// Read payload
		pl, err := ioutil.ReadAll(cfg.Stdin)
		if err != nil {
			log.WithError(err).Error(models.ErrInvalidPayload)
			c.JSON(http.StatusBadRequest, simpleError(models.ErrInvalidPayload))
			return true
		}
		// Create Task
		priority := int32(0)
		newTask.Priority = &priority
		newTask.Payload = string(pl)
		// Push to queue
		enqueue(c, s.MQ, newTask)
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

		for k, v := range found.Headers {
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
