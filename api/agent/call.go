package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/fnproject/fn/api/agent/drivers"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/models"
	"github.com/go-openapi/strfmt"
	"github.com/opentracing/opentracing-go"
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
	End(ctx context.Context, err error) error
}

// TODO build w/o closures... lazy
type CallOpt func(a *agent, c *call) error

type Param struct {
	Key   string
	Value string
}
type Params []Param

func FromRequest(appName, path string, req *http.Request, params Params) CallOpt {
	return func(a *agent, c *call) error {
		app, err := a.da.GetApp(req.Context(), appName)
		if err != nil {
			return err
		}

		route, err := a.da.GetRoute(req.Context(), appName, path)
		if err != nil {
			return err
		}

		if route.Format == "" {
			route.Format = "default"
		}

		id := id.New().String()

		// baseVars are the vars on the route & app, not on this specific request [for hot functions]
		baseVars := make(map[string]string, len(app.Config)+len(route.Config)+3)

		// add app & route config before our standard additions
		for k, v := range app.Config {
			k = toEnvName("", k)
			baseVars[k] = v
		}
		for k, v := range route.Config {
			k = toEnvName("", k)
			baseVars[k] = v
		}

		baseVars["FN_FORMAT"] = route.Format
		baseVars["FN_APP_NAME"] = appName
		baseVars["FN_PATH"] = route.Path
		// TODO: might be a good idea to pass in: envVars["FN_BASE_PATH"] = fmt.Sprintf("/r/%s", appName) || "/" if using DNS entries per app
		baseVars["FN_MEMORY"] = fmt.Sprintf("%d", route.Memory)
		baseVars["FN_TYPE"] = route.Type

		// envVars contains the full set of env vars, per request + base
		envVars := make(map[string]string, len(baseVars)+len(params)+len(req.Header)+3)

		for k, v := range baseVars {
			envVars[k] = v
		}

		envVars["FN_CALL_ID"] = id
		envVars["FN_METHOD"] = req.Method
		envVars["FN_REQUEST_URL"] = func() string {
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
		}()

		// params
		for _, param := range params {
			envVars[toEnvName("FN_PARAM", param.Key)] = param.Value
		}

		headerVars := make(map[string]string, len(req.Header))

		for k, v := range req.Header {
			if !noOverrideVars(k) { // NOTE if we don't do this, they'll leak in (don't want people relying on this behavior)
				headerVars[toEnvName("FN_HEADER", k)] = strings.Join(v, ", ")
			}
		}

		// add all the env vars we build to the request headers
		for k, v := range envVars {
			if noOverrideVars(k) {
				// overwrite the passed in request headers explicitly with the generated ones
				req.Header.Set(k, v)
			} else {
				req.Header.Add(k, v)
			}
		}

		for k, v := range headerVars {
			envVars[k] = v
		}

		// TODO this relies on ordering of opts, but tests make sure it works, probably re-plumb/destroy headers
		// TODO async should probably supply an http.ResponseWriter that records the logs, to attach response headers to
		if rw, ok := c.w.(http.ResponseWriter); ok {
			rw.Header().Add("FN_CALL_ID", id)
			for k, vs := range route.Headers {
				for _, v := range vs {
					// pre-write in these headers to response
					rw.Header().Add(k, v)
				}
			}
		}

		// this ensures that there is an image, path, timeouts, memory, etc are valid.
		// NOTE: this means assign any changes above into route's fields
		err = route.Validate()
		if err != nil {
			return err
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

		c.req = req
		return nil
	}
}

func noOverrideVars(key string) bool {
	// descrepency in casing b/w req headers and env vars, force matches
	return overrideVars[strings.ToUpper(key)]
}

// overrideVars means that the app config, route config or header vars
// must not overwrite the generated values in call construction.
var overrideVars = map[string]bool{
	"FN_FORMAT":      true,
	"FN_APP_NAME":    true,
	"FN_PATH":        true,
	"FN_MEMORY":      true,
	"FN_TYPE":        true,
	"FN_CALL_ID":     true,
	"FN_METHOD":      true,
	"FN_REQUEST_URL": true,
}

// TODO this currently relies on FromRequest having happened before to create the model
// here, to be a fully qualified model. We probably should double check but having a way
// to bypass will likely be what's used anyway unless forced.
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

	c.da = a.da
	c.ct = a

	ctx, _ := common.LoggerWithFields(c.req.Context(),
		logrus.Fields{"id": c.ID, "app": c.AppName, "route": c.Path})
	c.req = c.req.WithContext(ctx)

	// setup stderr logger separate (don't inherit ctx vars)
	logger := logrus.WithFields(logrus.Fields{"user_log": true, "app_name": c.AppName, "path": c.Path, "image": c.Image, "call_id": c.ID})
	c.stderr = setupLogger(logger)
	if c.w == nil {
		// send STDOUT to logs if no writer given (async...)
		// TODO we could/should probably make this explicit to GetCall, ala 'WithLogger', but it's dupe code (who cares?)
		c.w = c.stderr
	}

	return &c, nil
}

type call struct {
	*models.Call

	da     DataAccess
	w      io.Writer
	req    *http.Request
	stderr io.ReadWriteCloser
	ct     callTrigger
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

	if rw, ok := c.w.(http.ResponseWriter); ok { // TODO need to figure out better way to wire response headers in
		rw.Header().Set("XXX-FXLB-WAIT", time.Time(c.StartedAt).Sub(time.Time(c.CreatedAt)).String())
	}

	if c.Type == models.TypeAsync {
		// XXX (reed): make sure MQ reservation is lengthy. to skirt MQ semantics,
		// we could add a new message to MQ w/ delay of call.Timeout and delete the
		// old one (in that order), after marking the call as running in the db
		// (see below)

		// XXX (reed): should we store the updated started_at + status? we could
		// use this so that if we pick up a call from mq and find its status is
		// running to avoid running the call twice and potentially mark it as
		// errored (built in long running task detector, so to speak...)

		err := c.da.Start(ctx, c.Model())
		if err != nil {
			return err // let another thread try this
		}
	}

	err := c.ct.fireBeforeCall(ctx, c.Model())
	if err != nil {
		return fmt.Errorf("BeforeCall: %v", err)
	}

	return nil
}

func (c *call) End(ctx context.Context, errIn error) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "agent_call_end")
	defer span.Finish()

	c.CompletedAt = strfmt.DateTime(time.Now())

	switch errIn {
	case nil:
		c.Status = "success"
	case context.DeadlineExceeded:
		c.Status = "timeout"
	default:
		c.Status = "error"
		c.Error = errIn.Error()
	}

	// ensure stats histogram is reasonably bounded
	c.Call.Stats = drivers.Decimate(240, c.Call.Stats)

	if err := c.da.Finish(ctx, c.Model(), c.stderr, c.Type == models.TypeAsync); err != nil {
		common.Logger(ctx).WithError(err).Error("error finalizing call on datastore/mq")
		// note: Not returning err here since the job could have already finished successfully.
	}

	if err := c.ct.fireAfterCall(ctx, c.Model()); err != nil {
		return fmt.Errorf("AfterCall: %v", err)
	}

	return errIn // original error, important for use in sync call returns
}

func toEnvName(envtype, name string) string {
	name = strings.Replace(name, "-", "_", -1)
	if envtype == "" {
		return name
	}
	return fmt.Sprintf("%s_%s", envtype, name)
}
