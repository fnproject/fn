package server

import (
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"encoding/json"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/functions/api/runner"
)

func handleRunner(c *gin.Context) {
	if strings.HasPrefix(c.Request.URL.Path, "/v1") {
		c.Status(http.StatusNotFound)
		return
	}

	log := c.MustGet("log").(logrus.FieldLogger)

	var err error

	var payload []byte
	if c.Request.Method == "POST" || c.Request.Method == "PUT" {
		payload, err = ioutil.ReadAll(c.Request.Body)
	} else if c.Request.Method == "GET" {
		qPL := c.Request.URL.Query()["payload"]
		if len(qPL) > 0 {
			payload = []byte(qPL[0])
		}
	}

	if len(payload) > 0 {
		var emptyJSON map[string]interface{}
		if err := json.Unmarshal(payload, &emptyJSON); err != nil {
			log.WithError(err).Error(models.ErrInvalidJSON)
			c.JSON(http.StatusBadRequest, simpleError(models.ErrInvalidJSON))
			return
		}
	}

	log.WithField("payload", string(payload)).Debug("Got payload")

	appName := c.Param("app")
	if appName == "" {
		host := strings.Split(c.Request.Host, ":")[0]
		appName = strings.Split(host, ".")[0]
	}

	route := c.Param("route")
	if route == "" {
		route = c.Request.URL.Path
	}

	filter := &models.RouteFilter{
		Path:    route,
		AppName: appName,
	}

	log.WithFields(logrus.Fields{"app": appName, "path": route}).Debug("Finding route on datastore")

	routes, err := Api.Datastore.GetRoutes(filter)
	if err != nil {
		log.WithError(err).Error(models.ErrRoutesList)
		c.JSON(http.StatusInternalServerError, simpleError(models.ErrRoutesList))
	}

	if routes == nil || len(routes) == 0 {
		log.WithError(err).Error(models.ErrRunnerRouteNotFound)
		c.JSON(http.StatusNotFound, simpleError(models.ErrRunnerRouteNotFound))
	}

	log.WithField("routes", routes).Debug("Got routes from datastore")

	for _, el := range routes {
		if el.Path == route {
			run := runner.New(&runner.Config{
				Ctx:     c,
				Route:   el,
				Payload: string(payload),
				Timeout: 30 * time.Second,
			})

			if err := run.Run(); err != nil {
				log.WithError(err).Error(models.ErrRunnerRunRoute)
				c.JSON(http.StatusInternalServerError, simpleError(models.ErrRunnerRunRoute))
			} else {
				for k, v := range el.Headers {
					c.Header(k, v[0])
				}

				if run.Status() == "success" {
					c.Data(http.StatusOK, "", run.ReadOut())
				} else {
					c.Data(http.StatusInternalServerError, "", run.ReadErr())
				}
			}
			return
		}
	}

}
