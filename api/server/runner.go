package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"strconv"
	"sync"
	"time"

	"github.com/fnproject/cloudevent"
	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/agent/protocol"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

const (
	ceMimeType = "application/cloudevents+json"
)

// handleFunctionCall executes the function, for router handlers
func (s *Server) handleFunctionCall(c *gin.Context) {
	err := s.handleFunctionCall2(c)
	if err != nil {
		handleErrorResponse(c, err)
	}
}

// handleFunctionCall2 executes the function and returns an error
// Requires the following in the context:
// * "app_name"
// * "path"
func (s *Server) handleFunctionCall2(c *gin.Context) error {
	ctx := c.Request.Context()
	var p string
	r := ctx.Value(api.Path)
	if r == nil {
		p = "/"
	} else {
		p = r.(string)
	}

	appID := c.MustGet(api.AppID).(string)
	app, err := s.agent.GetAppByID(ctx, appID)
	if err != nil {
		return err
	}

	// gin sets this to 404 on NoRoute, so we'll just ensure it's 200 by default.
	c.Status(200) // this doesn't write the header yet

	return s.serve(c, app, path.Clean(p))
}

var (
	bufPool = &sync.Pool{New: func() interface{} { return new(bytes.Buffer) }}
)

// TODO it would be nice if we could make this have nothing to do with the gin.Context but meh
// TODO make async store an *http.Request? would be sexy until we have different api format...
func (s *Server) serve(c *gin.Context, app *models.App, path string) error {
	ctx := c.Request.Context()
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	writer := syncResponseWriter{
		Buffer:  buf,
		headers: c.Writer.Header(), // copy ref
	}
	defer bufPool.Put(buf) // TODO need to ensure this is safe with Dispatch?

	route, err := s.datastore.GetRoute(ctx, app.ID, path)
	if err != nil {
		return err
	}

	event, err := s.requestToEvent(app, route, path, c.Request)
	ctx, _ = common.LoggerWithFields(c.Request.Context(), logrus.Fields{"id": event.EventID})
	c.Request = c.Request.WithContext(ctx)

	// todo: this should be done in requestToEvent
	err = json.NewDecoder(c.Request.Body).Decode(&event.Data)
	if err != nil {
		return fmt.Errorf("Invalid json body with contentType 'application/json'. %v", err)
	}

	event, err = s.agent.Handle(ctx, event)
	if err != nil {
		// NOTE if they cancel the request then it will stop the call (kind of cool),
		// we could filter that error out here too as right now it yells a little
		if err == models.ErrCallTimeoutServerBusy || err == models.ErrCallTimeout {
			// TODO maneuver
			// add this, since it means that start may not have been called [and it's relevant]
			c.Writer.Header().Add("XXX-FXLB-WAIT", time.Now().Sub(*event.EventTime).String())
		}
		return err
	}
	// check if event was async and if so...
	if route.Type == "async" {
		c.JSON(http.StatusAccepted, map[string]string{"call_id": event.EventID})
		return nil
	}

	// if they don't set a content-type - detect it
	if writer.Header().Get("Content-Type") == "" {
		// see http.DetectContentType, the go server is supposed to do this for us but doesn't appear to?
		var contentType string
		jsonPrefix := [1]byte{'{'} // stack allocated
		if bytes.HasPrefix(buf.Bytes(), jsonPrefix[:]) {
			// try to detect json, since DetectContentType isn't a hipster.
			contentType = "application/json; charset=utf-8"
		} else {
			contentType = http.DetectContentType(buf.Bytes())
		}
		writer.Header().Set("Content-Type", contentType)
	}

	// TODO: Add FN_CALL_ID header

	writer.Header().Set("Content-Length", strconv.Itoa(int(buf.Len())))

	if writer.status > 0 {
		c.Writer.WriteHeader(writer.status)
	}
	io.Copy(c.Writer, &writer)

	return nil
}

var _ http.ResponseWriter = new(syncResponseWriter)

// implements http.ResponseWriter
// this little guy buffers responses from user containers and lets them still
// set headers and such without us risking writing partial output [as much, the
// server could still die while we're copying the buffer]. this lets us set
// content length and content type nicely, as a bonus. it is sad, yes.
type syncResponseWriter struct {
	headers http.Header
	status  int
	*bytes.Buffer
}

func (s *syncResponseWriter) Header() http.Header  { return s.headers }
func (s *syncResponseWriter) WriteHeader(code int) { s.status = code }

func (s *Server) requestToEvent(app *models.App, route *models.Route, path string, req *http.Request) (cloudevent.CloudEvent, error) {
	ctx := req.Context()

	log := common.Logger(ctx)
	event := cloudevent.CloudEvent{}

	// Check whether this is a CloudEvent, if coming in via HTTP router (only way currently), then we'll look for a special header
	// Content-Type header: https://github.com/cloudevents/spec/blob/master/http-transport-binding.md#32-structured-content-mode
	// Expected Content-Type for a CloudEvent: application/cloudevents+json; charset=UTF-8
	contentType := req.Header.Get("Content-Type")
	t, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		// won't fail here, but log
		log.Debugf("Could not parse Content-Type header: %v", err)
	} else {
		if t == ceMimeType { // it's already a cloud event
			// c.IsCloudEvent = true
			route.Format = models.FormatCloudEvent
			err = json.NewDecoder(req.Body).Decode(&event)
			if err != nil {
				return event, fmt.Errorf("Invalid CloudEvent input. %v", err)
			}
		}
	}

	if route.Format == "" {
		route.Format = models.FormatDefault
	}

	// todo: check that these things aren't filled in already?
	id := id.New().String()
	now := time.Now()
	event.EventID = id
	event.EventType = "http"
	event.Source = reqURL(req)
	event.EventTime = &now

	// this ensures that there is an image, path, timeouts, memory, etc are valid.
	// NOTE: this means assign any changes above into route's fields
	err = route.Validate()
	if err != nil {
		return event, err
	}

	exts := map[string]interface{}{}
	exts["appID"] = app.ID
	exts["function"] = map[string]interface{}{
		"path":        route.Path,
		"image":       route.Image,
		"type":        route.Type,
		"format":      route.Format,
		"priority":    new(int32),
		"timeout":     route.Timeout,
		"idleTimeout": route.IdleTimeout,
		"memory":      route.Memory,
		"cpus":        route.CPUs,
	}
	exts["protocol"] = protocol.CallRequestHTTP{
		Type:       "http",
		Method:     req.Method,
		RequestURL: req.URL.String(),
		Headers:    req.Header,
	}
	exts["config"] = buildConfig(app, route)
	exts["annotations"] = buildAnnotations(app, route)

	event.Extensions = exts

	return event, nil
}

func buildConfig(app *models.App, route *models.Route) models.Config {
	conf := make(models.Config, 8+len(app.Config)+len(route.Config))
	for k, v := range app.Config {
		conf[k] = v
	}
	for k, v := range route.Config {
		conf[k] = v
	}

	conf["FN_FORMAT"] = route.Format
	conf["FN_APP_NAME"] = app.Name
	conf["FN_PATH"] = route.Path
	// TODO: might be a good idea to pass in: "FN_BASE_PATH" = fmt.Sprintf("/r/%s", appName) || "/" if using DNS entries per app
	conf["FN_MEMORY"] = fmt.Sprintf("%d", route.Memory)
	conf["FN_TYPE"] = route.Type

	CPUs := route.CPUs.String()
	if CPUs != "" {
		conf["FN_CPUS"] = CPUs
	}
	return conf
}

func buildAnnotations(app *models.App, route *models.Route) models.Annotations {
	ann := make(models.Annotations, len(app.Annotations)+len(route.Annotations))
	for k, v := range app.Annotations {
		ann[k] = v
	}
	for k, v := range route.Annotations {
		ann[k] = v
	}
	return ann
}

func reqURL(req *http.Request) string {
	if req.URL.Scheme == "" {
		if req.TLS == nil {
			req.URL.Scheme = "http"
		} else {
			req.URL.Scheme = "https"
		}
	}
	if req.URL.Host == "" {
		req.URL.Host = req.Host
	}
	return req.URL.String()
}
