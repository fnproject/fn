package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/models"
	"github.com/go-openapi/strfmt"
	"github.com/opentracing/opentracing-go"
	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
)

type Call interface {
	// Model will return the underlying models.Call configuration for this call.
	// TODO we could respond to async correctly from agent but layering, this
	// is only because the front end has different responses based on call type.
	// try to discourage use elsewhere until this gets pushed down more...
	Model() *models.Call

	// Start will be called before this call is executed, it may be used to
	// guarantee mutual exclusion, check docker permissions, update timestamps,
	// etc.
	// TODO Start and End can likely be unexported as they are only used in the agent,
	// and on a type which is constructed in a specific agent. meh.
	Start(ctx context.Context) error

	// End will be called immediately after attempting a call execution,
	// regardless of whether the execution failed or not. An error will be passed
	// to End, which if nil indicates a successful execution. Any error returned
	// from End will be returned as the error from Submit.
	End(ctx context.Context, err error)
}

// TODO build w/o closures... lazy
type CallOpt func(a *agent, c *call) error

func FromRequest(appName, path string, req *http.Request) CallOpt {
	return func(a *agent, c *call) error {
		// TODO we need to add a little timeout to these 2 things
		app, err := a.app(req.Context(), appName)
		if err != nil {
			return err
		}

		route, err := a.route(req.Context(), appName, path)
		if err != nil {
			return err
		}

		params, match := matchRoute(route.Path, path)
		if !match {
			return errors.New("route does not match") // TODO wtf, can we ignore match?
		}

		if route.Format == "" {
			route.Format = "default"
		}

		id := id.New().String()

		// baseVars are the vars on the route & app, not on this specific request [for hot functions]
		baseVars := make(map[string]string, len(app.Config)+len(route.Config)+3)
		baseVars["FN_FORMAT"] = route.Format
		baseVars["APP_NAME"] = appName
		baseVars["ROUTE"] = route.Path
		baseVars["MEMORY_MB"] = fmt.Sprintf("%d", route.Memory)

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
		envVars := make(map[string]string, len(baseVars)+len(params)+len(req.Header)+3)

		for k, v := range baseVars {
			envVars[k] = v
		}

		envVars["CALL_ID"] = id
		envVars["METHOD"] = req.Method
		envVars["REQUEST_URL"] = fmt.Sprintf("%v://%v%v", func() string {
			if req.TLS == nil {
				return "http"
			}
			return "https"
		}(), req.Host, req.URL.String())

		// params
		for _, param := range params {
			envVars[toEnvName("PARAM", param.Key)] = param.Value
		}

		headerVars := make(map[string]string,len(req.Header))

		for k, v := range req.Header {
			headerVars[toEnvName("HEADER",k)] = strings.Join(v, ", ")
		}

		// add all the env vars we build to the request headers
		// TODO should we save req.Headers and copy OVER app.Config / route.Config ?
		for k, v := range envVars {
			req.Header.Add(k, v)
		}

		for k,v := range headerVars {
			envVars[k] = v
		}

		// TODO this relies on ordering of opts, but tests make sure it works, probably re-plumb/destroy headers
		if rw, ok := c.w.(http.ResponseWriter); ok {
			rw.Header().Add("FN_CALL_ID", id)
			for k, vs := range route.Headers {
				for _, v := range vs {
					// pre-write in these headers to response
					rw.Header().Add(k, v)
				}
			}
		}

		c.Call = &models.Call{
			ID:      id,
			AppName: appName,
			Path:    route.Path,
			Image:   route.Image,
			// Delay: 0,
			Type:   route.Type,
			Format: route.Format,
			// Payload: TODO,
			Priority:    new(int32), // TODO this is crucial, apparently
			Timeout:     route.Timeout,
			IdleTimeout: route.IdleTimeout,
			Memory:      route.Memory,
			BaseEnv:     baseVars,
			EnvVars:     envVars,
			CreatedAt:   strfmt.DateTime(time.Now()),
			URL:         req.URL.String(), // TODO we should probably strip host/port
			Method:      req.Method,
		}

		// TODO if these made it to here we have a problemo. error instead?
		if c.Timeout <= 0 {
			c.Timeout = models.DefaultRouteTimeout
		}
		if c.IdleTimeout <= 0 {
			c.IdleTimeout = models.DefaultIdleTimeout
		}

		c.req = req
		return nil
	}
}

func FromModel(mCall *models.Call) CallOpt {
	return func(a *agent, c *call) error {
		c.Call = mCall

		// NOTE this adds content length based on payload length
		req, err := http.NewRequest(c.Method, c.URL, strings.NewReader(c.Payload))
		if err != nil {
			return err
		}
		for k, v := range c.EnvVars {
			// TODO if we don't store env as []string headers are messed up
			req.Header.Set(k, v)
		}

		c.req = req
		// TODO anything else really?
		return nil
	}
}

// TODO this should be required
func WithWriter(w io.Writer) CallOpt {
	return func(a *agent, c *call) error {
		c.w = w
		return nil
	}
}

// GetCall builds a Call that can be used to submit jobs to the agent.
//
// TODO we could make this package level just moving the cache around. meh.
// TODO where to put this? async and sync both call this
func (a *agent) GetCall(opts ...CallOpt) (Call, error) {
	var c call

	for _, o := range opts {
		err := o(a, &c)
		if err != nil {
			return nil, err
		}
	}

	// TODO typed errors to test
	if c.req == nil || c.Call == nil {
		return nil, errors.New("no model or request provided for call")
	}

	// TODO move func logger here
	// TODO add log store interface (yagni?)
	c.ds = a.ds
	c.mq = a.mq

	return &c, nil
}

type call struct {
	*models.Call

	ds     models.Datastore
	mq     models.MessageQueue
	w      io.Writer
	req    *http.Request
	stderr io.WriteCloser
}

func (c *call) Model() *models.Call { return c.Call }

func (c *call) Start(ctx context.Context) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "agent_call_start")
	defer span.Finish()

	// TODO discuss this policy. cold has not yet started the container,
	// hot just has to dispatch
	//
	// make sure we have at least half our timeout to run, or timeout here
	deadline, ok := ctx.Deadline()
	need := time.Now().Add(time.Duration(c.Timeout) * time.Second) // > deadline, always
	// need.Sub(deadline) = elapsed time
	if ok && need.Sub(deadline) > (time.Duration(c.Timeout)*time.Second)/2 {
		return context.DeadlineExceeded
	}

	c.StartedAt = strfmt.DateTime(time.Now())
	c.Status = "running"

	if c.Type == models.TypeAsync {
		// XXX (reed): make sure MQ reservation is lengthy. to skirt MQ semantics,
		// we could add a new message to MQ w/ delay of call.Timeout and delete the
		// old one (in that order), after marking the call as running in the db
		// (see below)

		// XXX (reed): should we store the updated started_at + status? we could
		// use this so that if we pick up a call from mq and find its status is
		// running to avoid running the call twice and potentially mark it as
		// errored (built in long running task detector, so to speak...)

		err := c.mq.Delete(ctx, c.Call)
		if err != nil {
			return err // let another thread try this
		}
	}
	return nil
}

func (c *call) End(ctx context.Context, err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "agent_call_end")
	defer span.Finish()

	c.CompletedAt = strfmt.DateTime(time.Now())

	switch err {
	case nil:
		c.Status = "success"
	case context.DeadlineExceeded:
		c.Status = "timeout"
	default:
		// XXX (reed): should we append the error to logs? Error field?
		c.Status = "error"
	}

	if c.Type == models.TypeAsync {
		// XXX (reed): delete MQ message, eventually
	}

	// need newer context because original one may be modified
	// and no longer be valid for further context-bound operations
	if err := c.ds.InsertCall(context.Background(), c.Call); err != nil {
		// TODO we should just log this error not return it to user?
		// just issue storing call status but call is run
		logrus.WithError(err).Error("error inserting call into datastore")
	}
}

func (a *agent) route(ctx context.Context, appName, path string) (*models.Route, error) {
	key := routeCacheKey(appName, path)
	route, ok := a.cache.Get(key)
	if ok {
		return route.(*models.Route), nil
	}

	resp, err := a.singleflight.Do(key,
		func() (interface{}, error) { return a.ds.GetRoute(ctx, appName, path) },
	)
	if err != nil {
		return nil, err
	}
	route = resp.(*models.Route)
	a.cache.Set(key, route, cache.DefaultExpiration)
	return route.(*models.Route), nil
}

func (a *agent) app(ctx context.Context, appName string) (*models.App, error) {
	key := appCacheKey(appName)
	app, ok := a.cache.Get(key)
	if ok {
		return app.(*models.App), nil
	}

	resp, err := a.singleflight.Do(key,
		func() (interface{}, error) { return a.ds.GetApp(ctx, appName) },
	)
	if err != nil {
		return nil, err
	}
	app = resp.(*models.App)
	a.cache.Set(key, app, cache.DefaultExpiration)
	return app.(*models.App), nil
}

func routeCacheKey(appname, path string) string {
	return "r:" + appname + "\x00" + path
}

func appCacheKey(appname string) string {
	return "a:" + appname
}

func fakeHandler(http.ResponseWriter, *http.Request, Params) {}

// TODO what is this stuff anyway?
func matchRoute(baseRoute, route string) (Params, bool) {
	tree := &node{}
	tree.addRoute(baseRoute, fakeHandler)
	handler, p, _ := tree.getValue(route)
	if handler == nil {
		return nil, false
	}

	return p, true
}

func toEnvName(envtype, name string) string {
	name = strings.Replace(name, "-", "_", -1)
	if envtype == "" {
		return name
	}
	return fmt.Sprintf("%s_%s", envtype, name)
}
