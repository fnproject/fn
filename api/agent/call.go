package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"go.opencensus.io/trace"

	"github.com/fnproject/fn/api/agent/drivers"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/models"
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

// Interceptor in GetCall
type CallOverrider func(*models.Call, map[string]string) (map[string]string, error)

// TODO build w/o closures... lazy
type CallOpt func(c *call) error

const (
	ceMimeType = "application/cloudevents+json"
)

// FromRequest initialises a call to a route from an HTTP request
// deprecate with routes
func FromRequest(app *models.App, route *models.Route, req *http.Request) CallOpt {
	return func(c *call) error {
		ctx := req.Context()

		log := common.Logger(ctx)
		// Check whether this is a CloudEvent, if coming in via HTTP router (only way currently), then we'll look for a special header
		// Content-Type header: https://github.com/cloudevents/spec/blob/master/http-transport-binding.md#32-structured-content-mode
		// Expected Content-Type for a CloudEvent: application/cloudevents+json; charset=UTF-8
		contentType := req.Header.Get("Content-Type")
		t, _, err := mime.ParseMediaType(contentType)
		if err != nil {
			// won't fail here, but log
			log.Debugf("Could not parse Content-Type header: %v", err)
		} else {
			if t == ceMimeType {
				c.IsCloudEvent = true
				route.Format = models.FormatCloudEvent
			}
		}

		if route.Format == "" {
			route.Format = models.FormatDefault
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

		var syslogURL string
		if app.SyslogURL != nil {
			syslogURL = *app.SyslogURL
		}

		c.Call = &models.Call{
			ID:    id,
			Path:  route.Path,
			Image: route.Image,
			// Delay: 0,
			Type:   route.Type,
			Format: route.Format,
			// Payload: TODO,
			Priority:    new(int32), // TODO this is crucial, apparently
			Timeout:     route.Timeout,
			IdleTimeout: route.IdleTimeout,
			TmpFsSize:   route.TmpFsSize,
			Memory:      route.Memory,
			CPUs:        route.CPUs,
			Config:      buildConfig(app, route),
			Annotations: app.Annotations.MergeChange(route.Annotations),
			Headers:     req.Header,
			CreatedAt:   common.DateTime(time.Now()),
			URL:         reqURL(req),
			Method:      req.Method,
			AppID:       app.ID,
			SyslogURL:   syslogURL,
		}

		c.req = req
		return nil
	}
}

// Sets up a call from an http trigger request
func FromHTTPTriggerRequest(app *models.App, fn *models.Fn, trigger *models.Trigger, req *http.Request) CallOpt {
	return func(c *call) error {
		ctx := req.Context()

		log := common.Logger(ctx)
		// Check whether this is a CloudEvent, if coming in via HTTP router (only way currently), then we'll look for a special header
		// Content-Type header: https://github.com/cloudevents/spec/blob/master/http-transport-binding.md#32-structured-content-mode
		// Expected Content-Type for a CloudEvent: application/cloudevents+json; charset=UTF-8
		contentType := req.Header.Get("Content-Type")
		t, _, err := mime.ParseMediaType(contentType)
		if err != nil {
			// won't fail here, but log
			log.Debugf("Could not parse Content-Type header: %v", err)
		} else {
			if t == ceMimeType {
				c.IsCloudEvent = true
				fn.Format = models.FormatCloudEvent
			}
		}

		if fn.Format == "" {
			fn.Format = models.FormatDefault
		}

		id := id.New().String()

		// TODO this relies on ordering of opts, but tests make sure it works, probably re-plumb/destroy headers
		// TODO async should probably supply an http.ResponseWriter that records the logs, to attach response headers to
		if rw, ok := c.w.(http.ResponseWriter); ok {
			rw.Header().Add("FN_CALL_ID", id)
		}

		var syslogURL string
		if app.SyslogURL != nil {
			syslogURL = *app.SyslogURL
		}

		c.Call = &models.Call{
			ID:    id,
			Path:  trigger.Source,
			Image: fn.Image,
			// Delay: 0,
			Type:   "sync",
			Format: fn.Format,
			// Payload: TODO,
			Priority:    new(int32), // TODO this is crucial, apparently
			Timeout:     fn.Timeout,
			IdleTimeout: fn.IdleTimeout,
			TmpFsSize:   0, // TODO clean up this
			Memory:      fn.Memory,
			CPUs:        0, // TODO clean up this
			Config:      buildTriggerConfig(app, fn, trigger),
			// TODO - this wasn't really the intention here (that annotations would naturally cascade
			// but seems to be necessary for some runner behaviour
			Annotations: app.Annotations.MergeChange(fn.Annotations).MergeChange(trigger.Annotations),
			Headers:     req.Header,
			CreatedAt:   common.DateTime(time.Now()),
			URL:         reqURL(req),
			Method:      req.Method,
			AppID:       app.ID,
			FnID:        fn.ID,
			TriggerID:   trigger.ID,
			SyslogURL:   syslogURL,
		}

		c.req = req
		return nil
	}
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
	conf["FN_TMPSIZE"] = fmt.Sprintf("%d", route.TmpFsSize)

	CPUs := route.CPUs.String()
	if CPUs != "" {
		conf["FN_CPUS"] = CPUs
	}
	return conf
}

func buildTriggerConfig(app *models.App, fn *models.Fn, trigger *models.Trigger) models.Config {
	conf := make(models.Config, 8+len(app.Config)+len(fn.Config))
	for k, v := range app.Config {
		conf[k] = v
	}
	for k, v := range fn.Config {
		conf[k] = v
	}

	conf["FN_FORMAT"] = fn.Format
	conf["FN_APP_NAME"] = app.Name
	conf["FN_PATH"] = trigger.Source
	// TODO: might be a good idea to pass in: "FN_BASE_PATH" = fmt.Sprintf("/r/%s", appName) || "/" if using DNS entries per app
	conf["FN_MEMORY"] = fmt.Sprintf("%d", fn.Memory)
	conf["FN_TYPE"] = "sync"
	conf["FN_FN_ID"] = fn.ID

	return conf
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

// FromModel creates a call object from an existing stored call model object, reading the body from the stored call payload
func FromModel(mCall *models.Call) CallOpt {
	return func(c *call) error {
		c.Call = mCall

		req, err := http.NewRequest(c.Method, c.URL, strings.NewReader(c.Payload))
		if err != nil {
			return err
		}
		req.Header = c.Headers

		c.req = req
		// TODO anything else really?
		return nil
	}
}

// FromModelAndInput creates a call object from an existing stored call model object , reading the body from a provided stream
func FromModelAndInput(mCall *models.Call, in io.ReadCloser) CallOpt {
	return func(c *call) error {
		c.Call = mCall

		req, err := http.NewRequest(c.Method, c.URL, in)
		if err != nil {
			return err
		}
		req.Header = c.Headers

		c.req = req
		// TODO anything else really?
		return nil
	}
}

// WithWriter sets the writier that the call uses to send its output message to
// TODO this should be required
func WithWriter(w io.Writer) CallOpt {
	return func(c *call) error {
		c.w = w
		return nil
	}
}

// WithContext overrides the context on the call
func WithContext(ctx context.Context) CallOpt {
	return func(c *call) error {
		c.req = c.req.WithContext(ctx)
		return nil
	}
}

// WithExtensions adds internal attributes to the call that can be interpreted by extensions in the agent
// Pure runner can use this to pass an extension to the call
func WithExtensions(extensions map[string]string) CallOpt {
	return func(c *call) error {
		c.extensions = extensions
		return nil
	}
}

// GetCall builds a Call that can be used to submit jobs to the agent.
//
// TODO where to put this? async and sync both call this
func (a *agent) GetCall(opts ...CallOpt) (Call, error) {
	var c call

	for _, o := range opts {
		err := o(&c)
		if err != nil {
			return nil, err
		}
	}

	// TODO typed errors to test
	if c.req == nil || c.Call == nil {
		return nil, errors.New("no model or request provided for call")
	}

	// If overrider is present, let's allow it to modify models.Call
	// and call extensions
	if a.callOverrider != nil {
		ext, err := a.callOverrider(c.Call, c.extensions)
		if err != nil {
			return nil, err
		}
		c.extensions = ext
	}

	mem := c.Memory + uint64(c.TmpFsSize)
	if !a.resources.IsResourcePossible(mem, uint64(c.CPUs), c.Type == models.TypeAsync) {
		// if we're not going to be able to run this call on this machine, bail here.
		return nil, models.ErrCallTimeoutServerBusy
	}

	setupCtx(&c)

	c.handler = a.da
	c.ct = a
	c.stderr = setupLogger(c.req.Context(), a.cfg.MaxLogSize, c.Call)
	if c.w == nil {
		// send STDOUT to logs if no writer given (async...)
		// TODO we could/should probably make this explicit to GetCall, ala 'WithLogger', but it's dupe code (who cares?)
		c.w = c.stderr
	}

	return &c, nil
}

func setupCtx(c *call) {
	ctx, _ := common.LoggerWithFields(c.req.Context(),
		logrus.Fields{"id": c.ID, "app_id": c.AppID, "route": c.Path})
	c.req = c.req.WithContext(ctx)
}

type call struct {
	*models.Call

	// IsCloudEvent flag whether this was ingested as a cloud event. This may become the default or only way.
	IsCloudEvent bool `json:"is_cloud_event"`

	handler        CallHandler
	w              io.Writer
	req            *http.Request
	stderr         io.ReadWriteCloser
	ct             callTrigger
	slots          *slotQueue
	requestState   RequestState
	containerState ContainerState
	slotHashId     string
	isLB           bool

	// LB & Pure Runner Extra Config
	extensions map[string]string
}

// SlotHashId returns a string identity for this call that can be used to uniquely place the call in a given container
// This should correspond to a unique identity (including data changes) of the underlying function
func (c *call) SlotHashId() string {
	return c.slotHashId
}

func (c *call) Extensions() map[string]string {
	return c.extensions
}

func (c *call) RequestBody() io.ReadCloser {
	if c.req.Body != nil && c.req.GetBody != nil {
		rdr, err := c.req.GetBody()
		if err == nil {
			return rdr
		}
	}
	return c.req.Body
}

func (c *call) ResponseWriter() http.ResponseWriter {
	return c.w.(http.ResponseWriter)
}

func (c *call) StdErr() io.ReadWriteCloser {
	return c.stderr
}

func (c *call) Model() *models.Call { return c.Call }

func (c *call) Start(ctx context.Context) error {
	ctx, span := trace.StartSpan(ctx, "agent_call_start")
	defer span.End()

	// Check context timeouts, errors
	if ctx.Err() != nil {
		return ctx.Err()
	}

	c.StartedAt = common.DateTime(time.Now())
	c.Status = "running"

	if !c.isLB {
		if rw, ok := c.w.(http.ResponseWriter); ok { // TODO need to figure out better way to wire response headers in
			rw.Header().Set("XXX-FXLB-WAIT", time.Time(c.StartedAt).Sub(time.Time(c.CreatedAt)).String())
		}
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

		err := c.handler.Start(ctx, c.Model())
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
	ctx, span := trace.StartSpan(ctx, "agent_call_end")
	defer span.End()

	c.CompletedAt = common.DateTime(time.Now())

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

	if err := c.handler.Finish(ctx, c.Model(), c.stderr, c.Type == models.TypeAsync); err != nil {
		common.Logger(ctx).WithError(err).Error("error finalizing call on datastore/mq")
		// note: Not returning err here since the job could have already finished successfully.
	}

	// NOTE call this after InsertLog or the buffer will get reset
	c.stderr.Close()

	if err := c.ct.fireAfterCall(ctx, c.Model()); err != nil {
		return fmt.Errorf("AfterCall: %v", err)
	}

	return errIn // original error, important for use in sync call returns
}
