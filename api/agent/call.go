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
	Start(ctx context.Context, t callTrigger) error

	// End will be called immediately after attempting a call execution,
	// regardless of whether the execution failed or not. An error will be passed
	// to End, which if nil indicates a successful execution. Any error returned
	// from End will be returned as the error from Submit.
	End(ctx context.Context, err error, t callTrigger) error
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
		app, err := a.ds.GetApp(req.Context(), appName)
		if err != nil {
			return err
		}

		route, err := a.ds.GetRoute(req.Context(), appName, path)
		if err != nil {
			return err
		}

		if route.Format == "" {
			route.Format = "default"
		}

		id := id.New().String()

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

		// build headers and re-set the request ones w/ additions
		headers := buildEnv(id, params, req, app, route)
		req.Header = headers.HTTP()

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
			Env:         headers,
			CreatedAt:   strfmt.DateTime(time.Now()),
			URL:         req.URL.String(), // TODO we should probably strip host/port
			Method:      req.Method,
		}

		c.req = req
		return nil
	}
}

func buildEnv(id string, params Params, req *http.Request, app *models.App, route *models.Route) *models.CallEnv {
	env := models.EnvFromReq(req)

	// TODO do we need to assert these all pass httplex.ValidHeaderFieldValue
	for k, v := range app.Config {
		env.AddBase(k, v)
	}
	for k, v := range route.Config {
		env.AddBase(k, v)
	}

	env.SetBase("FN_FORMAT", route.Format)
	env.SetBase("FN_APP_NAME", app.Name)
	env.SetBase("FN_PATH", route.Path)
	// TODO: might be a good idea to pass in: "FN_BASE_PATH" = fmt.Sprintf("/r/%s", appName) || "/" if using DNS entries per app
	env.SetBase("FN_MEMORY", fmt.Sprintf("%d", route.Memory))
	env.SetBase("FN_TYPE", route.Type)

	// these [could] change every request, not base
	env.Set("FN_CALL_ID", id)
	env.Set("FN_METHOD", req.Method)
	env.Set("FN_REQUEST_URL", reqURL(req))

	// params
	// TODO nobody knows why these exist yet
	for _, param := range params {
		env.Set("FN_PARAM_"+param.Key, param.Value)
	}

	return env
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
		req.Header = c.Env.HTTP()

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

	c.ds = a.ds
	c.ls = a.ls
	c.mq = a.mq

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

	ds     models.Datastore
	ls     models.LogStore
	mq     models.MessageQueue
	w      io.Writer
	req    *http.Request
	stderr io.ReadWriteCloser
}

func (c *call) Model() *models.Call { return c.Call }

func (c *call) Start(ctx context.Context, t callTrigger) error {
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

		err := c.mq.Delete(ctx, c.Call)
		if err != nil {
			return err // let another thread try this
		}
	}

	err := t.fireBeforeCall(ctx, c.Model())
	if err != nil {
		return fmt.Errorf("BeforeCall: %v", err)
	}

	return nil
}

func (c *call) End(ctx context.Context, errIn error, t callTrigger) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "agent_call_end")
	defer span.Finish()

	c.CompletedAt = strfmt.DateTime(time.Now())

	switch errIn {
	case nil:
		c.Status = "success"
	case context.DeadlineExceeded:
		c.Status = "timeout"
	default:
		// XXX (reed): should we append the error to logs? Error field? (TR) yes, think so, otherwise it's lost looks like?
		c.Status = "error"
	}

	if c.Type == models.TypeAsync {
		// XXX (reed): delete MQ message, eventually
	}

	// ensure stats histogram is reasonably bounded
	c.Call.Stats = drivers.Decimate(240, c.Call.Stats)

	// this means that we could potentially store an error / timeout status for a
	// call that ran successfully [by a user's perspective]
	// TODO: this should be update, really
	if err := c.ds.InsertCall(ctx, c.Call); err != nil {
		common.Logger(ctx).WithError(err).Error("error inserting call into datastore")
		// note: Not returning err here since the job could have already finished successfully.
	}

	if err := c.ls.InsertLog(ctx, c.AppName, c.ID, c.stderr); err != nil {
		common.Logger(ctx).WithError(err).Error("error uploading log")
		// note: Not returning err here since the job could have already finished successfully.
	}

	// NOTE call this after InsertLog or the buffer will get reset
	c.stderr.Close()
	err := t.fireAfterCall(ctx, c.Model())
	if err != nil {
		return fmt.Errorf("AfterCall: %v", err)
	}
	return errIn
}
