package server

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/context"

	"encoding/json"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/functions/api/runner"
	titancommon "github.com/iron-io/titan/common"
	"github.com/satori/go.uuid"
)

func handleSpecial(c *gin.Context) {
	ctx := c.MustGet("ctx").(context.Context)
	log := titancommon.Logger(ctx)

	err := Api.UseSpecialHandlers(c)
	if err != nil {
		log.WithError(err).Errorln("Error using special handler!")
		// todo: what do we do here? Should probably return a 500 or something
	}
}

func handleRunner(c *gin.Context) {
	if strings.HasPrefix(c.Request.URL.Path, "/v1") {
		c.Status(http.StatusNotFound)
		return
	}

	ctx := c.MustGet("ctx").(context.Context)
	log := titancommon.Logger(ctx)

	reqID := uuid.NewV5(uuid.Nil, fmt.Sprintf("%s%s%d", c.Request.RemoteAddr, c.Request.URL.Path, time.Now().Unix())).String()
	c.Set("reqID", reqID) // todo: put this in the ctx instead of gin's

	log = log.WithFields(logrus.Fields{"request_id": reqID})

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

	filter := &models.RouteFilter{
		Path: route,
	}

	log.WithFields(logrus.Fields{"app": appName, "path": route}).Debug("Finding route on datastore")

	routes, err := Api.Datastore.GetRoutesByApp(appName, filter)
	if err != nil {
		log.WithError(err).Error(models.ErrRoutesList)
		c.JSON(http.StatusInternalServerError, simpleError(models.ErrRoutesList))
		return
	}

	if routes == nil || len(routes) == 0 {
		log.WithError(err).Error(models.ErrRunnerRouteNotFound)
		c.JSON(http.StatusNotFound, simpleError(models.ErrRunnerRouteNotFound))
		return
	}

	log.WithField("routes", routes).Debug("Got routes from datastore")
	for _, el := range routes {
		if el.Path == route {
			run := runner.New(&runner.Config{
				Ctx:        c,
				Route:      el,
				Payload:    string(payload),
				Timeout:    30 * time.Second,
				ID:         reqID,
				RequestURL: c.Request.URL.String(),
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
