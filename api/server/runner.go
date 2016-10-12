package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/functions/api/runner"
	"github.com/iron-io/runner/common"
	"github.com/iron-io/runner/drivers"
	uuid "github.com/satori/go.uuid"
)

func handleSpecial(c *gin.Context) {
	ctx := c.MustGet("ctx").(context.Context)
	log := common.Logger(ctx)

	err := Api.UseSpecialHandlers(c)
	if err != nil {
		log.WithError(err).Errorln("Error using special handler!")
		// todo: what do we do here? Should probably return a 500 or something
	}
}

func toEnvName(envtype, name string) string {
	name = strings.ToUpper(strings.Replace(name, "-", "_", -1))
	return fmt.Sprintf("%s_%s", envtype, name)
}

func handleRequest(c *gin.Context, enqueue models.Enqueue) {
	if strings.HasPrefix(c.Request.URL.Path, "/v1") {
		c.Status(http.StatusNotFound)
		return
	}

	ctx := c.MustGet("ctx").(context.Context)

	reqID := uuid.NewV5(uuid.Nil, fmt.Sprintf("%s%s%d", c.Request.RemoteAddr, c.Request.URL.Path, time.Now().Unix())).String()
	c.Set("reqID", reqID) // todo: put this in the ctx instead of gin's

	ctx, log := common.LoggerWithFields(ctx, logrus.Fields{"call_id": reqID})

	var err error
	var payload io.Reader

	if c.Request.Method == "POST" || c.Request.Method == "PUT" {
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

	appName := c.Param("app")
	if appName == "" {
		// check context, app can be added via special handlers
		a, ok := c.Get("app")
		if ok {
			appName = a.(string)
		}
	}
	// if still no appName, we gotta exit
	if appName == "" {
		log.WithError(err).Error(models.ErrAppsNotFound)
		c.JSON(http.StatusBadRequest, simpleError(models.ErrAppsNotFound))
		return
	}
	route := c.Param("route")
	if route == "" {
		route = c.Request.URL.Path
	}

	log.WithFields(logrus.Fields{"app": appName, "path": route}).Debug("Finding route on datastore")

	app, err := Api.Datastore.GetApp(appName)
	if err != nil || app == nil {
		log.WithError(err).Error(models.ErrAppsNotFound)
		c.JSON(http.StatusNotFound, simpleError(models.ErrAppsNotFound))
		return
	}

	routes, err := Api.Datastore.GetRoutesByApp(appName, &models.RouteFilter{})
	if err != nil {
		log.WithError(err).Error(models.ErrRoutesList)
		c.JSON(http.StatusInternalServerError, simpleError(models.ErrRoutesList))
		return
	}

	log.WithField("routes", routes).Debug("Got routes from datastore")
	for _, el := range routes {
		log = log.WithFields(logrus.Fields{
			"app": appName, "route": el.Path, "image": el.Image})

		if params, match := matchRoute(el.Path, route); match {

			var stdout bytes.Buffer // TODO: should limit the size of this, error if gets too big. akin to: https://golang.org/pkg/io/#LimitReader
			stderr := runner.NewFuncLogger(appName, route, el.Image, reqID)

			envVars := map[string]string{
				"METHOD":      c.Request.Method,
				"ROUTE":       el.Path,
				"REQUEST_URL": c.Request.URL.String(),
			}

			// app config
			for k, v := range app.Config {
				envVars[toEnvName("CONFIG", k)] = v
			}

			// route config
			for k, v := range el.Config {
				envVars[toEnvName("CONFIG", k)] = v
			}

			// params
			for _, param := range params {
				envVars[toEnvName("PARAM", param.Key)] = param.Value
			}

			// headers
			for header, value := range c.Request.Header {
				envVars[toEnvName("HEADER", header)] = strings.Join(value, " ")
			}

			cfg := &runner.Config{
				Image:   el.Image,
				Timeout: 30 * time.Second,
				ID:      reqID,
				AppName: appName,
				Stdout:  &stdout,
				Stderr:  stderr,
				Env:     envVars,
				Memory:  el.Memory,
				Stdin:   payload,
			}

			var err error
			var result drivers.RunResult
			switch el.Type {
			case "async":
				// Read payload
				pl, err := ioutil.ReadAll(cfg.Stdin)
				if err != nil {
					log.WithError(err).Error(models.ErrInvalidPayload)
					c.JSON(http.StatusBadRequest, simpleError(models.ErrInvalidPayload))
					return
				}

				// Create Task
				priority := int32(0)
				task := &models.Task{}
				task.Image = &cfg.Image
				task.ID = cfg.ID
				task.Path = el.Path
				task.AppName = cfg.AppName
				task.Priority = &priority
				task.EnvVars = cfg.Env
				task.Payload = string(pl)
				// Push to queue
				enqueue(task)
				log.Info("Added new task to queue")

			default:
				if result, err = Api.Runner.Run(c, cfg); err != nil {
					break
				}
				for k, v := range el.Headers {
					c.Header(k, v[0])
				}

				if result.Status() == "success" {
					c.Data(http.StatusOK, "", stdout.Bytes())
				} else {

					c.AbortWithStatus(http.StatusInternalServerError)
				}
			}

			return
		}
	}

	log.WithError(err).Error(models.ErrRunnerRouteNotFound)
	c.JSON(http.StatusNotFound, simpleError(models.ErrRunnerRouteNotFound))
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
