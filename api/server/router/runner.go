package router

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/functions/api/runner"
)

func handleRunner(c *gin.Context) {
	log := c.MustGet("log").(logrus.FieldLogger)
	store := c.MustGet("store").(models.Datastore)
	config := c.MustGet("config").(*models.Config)

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

	log.WithField("payload", string(payload)).Debug("Got payload")

	appName := c.Param("app")
	if appName == "" {
		host := strings.Split(c.Request.Header.Get("Host"), ":")[0]
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

	routes, err := store.GetRoutes(filter)
	if err != nil {
		log.WithError(err).Error(models.ErrRoutesList)
		c.JSON(http.StatusInternalServerError, simpleError(models.ErrRoutesList))
	}

	log.WithField("routes", routes).Debug("Got routes from datastore")

	for _, el := range routes {
		if el.Path == route {
			titanJob := runner.CreateTitanJob(&runner.RouteRunner{
				Route:    el,
				Endpoint: config.API,
				Payload:  string(payload),
				Timeout:  30 * time.Second,
			})

			if err := titanJob.Wait(); err != nil {
				log.WithError(err).Error(models.ErrRunnerRunRoute)
				c.JSON(http.StatusInternalServerError, simpleError(models.ErrRunnerRunRoute))
			} else {
				for k, v := range el.Headers {
					c.Header(k, v[0])
				}

				c.Data(http.StatusOK, "", bytes.Trim(titanJob.Result(), "\x00"))
			}
			return
		}
	}

}
