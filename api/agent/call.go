package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/fnproject/fn/api/agent/drivers/docker"
	"github.com/fnproject/fn/api/agent/drivers/stats"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/models"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"
)

// Call is an agent specific instance of a call object, that is runnable.
type Call interface {
	// Model will return the underlying models.Call configuration for this call.
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

// CallOverrider should die. Interceptor in GetCall
type CallOverrider func(*http.Request, *models.Call, map[string]string) (map[string]string, error)

// CallOpt allows configuring a call before execution
// TODO(reed): consider the interface here, all options must be defined in agent and flexible
// enough for usage by extenders of fn, this straddling is painful. consider models.Call?
type CallOpt func(c *call) error

// FromHTTPFnRequest Sets up a call from an http trigger request
func FromHTTPFnRequest(app *models.App, fn *models.Fn, req *http.Request) CallOpt {
	return func(c *call) error {
		id := id.New().String()

		var syslogURL string
		if app.SyslogURL != nil {
			syslogURL = *app.SyslogURL
		}

		c.Call = &models.Call{
			ID:    id,
			Image: fn.Image,
			// Delay: 0,
			Type:        models.TypeSync,
			Timeout:     fn.Timeout,
			IdleTimeout: fn.IdleTimeout,
			TmpFsSize:   0, // TODO clean up this
			Memory:      fn.Memory,
			CPUs:        0, // TODO clean up this
			Config:      buildConfig(app, fn),
			// TODO - this wasn't really the intention here (that annotations would naturally cascade
			// but seems to be necessary for some runner behaviour
			Annotations: app.Annotations.MergeChange(fn.Annotations),
			Headers:     req.Header,
			CreatedAt:   common.DateTime(time.Now()),
			URL:         reqURL(req),
			Method:      req.Method,
			AppID:       app.ID,
			AppName:     app.Name,
			FnID:        fn.ID,
			SyslogURL:   syslogURL,
		}

		c.req = req
		return nil
	}
}

func buildConfig(app *models.App, fn *models.Fn) models.Config {
	conf := make(models.Config, 8+len(app.Config)+len(fn.Config))
	for k, v := range app.Config {
		conf[k] = v
	}
	for k, v := range fn.Config {
		conf[k] = v
	}

	// XXX(reed): add trigger id to request headers on call?

	conf["FN_MEMORY"] = fmt.Sprintf("%d", fn.Memory)
	conf["FN_TYPE"] = "sync"
	conf["FN_FN_ID"] = fn.ID
	conf["FN_APP_ID"] = app.ID

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

// WithTrigger adds trigger specific bits to a call.
// TODO consider removal, this is from a shuffle
func WithTrigger(t *models.Trigger) CallOpt {
	return func(c *call) error {
		// right now just set the trigger id
		c.TriggerID = t.ID
		return nil
	}
}

// WithWriter sets the writer that the call uses to send its output message to
// TODO this should be required
func WithWriter(w io.Writer) CallOpt {
	return func(c *call) error {
		c.respWriter = w
		return nil
	}
}

// WithLogger sets stderr to the provided one
func WithLogger(w io.ReadWriteCloser) CallOpt {
	return func(c *call) error {
		c.stderr = w
		return nil
	}
}

// InvokeDetached mark a call to be a detached call
func InvokeDetached() CallOpt {
	return func(c *call) error {
		c.Model().Type = models.TypeDetached
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

// WithDockerAuth configures a call to retrieve credentials for an image pull
func WithDockerAuth(auth docker.Auther) CallOpt {
	return func(c *call) error {
		c.dockerAuth = auth
		return nil
	}
}

// GetCall builds a Call that can be used to submit jobs to the agent.
func (a *agent) GetCall(opts ...CallOpt) (Call, error) {
	var c call

	// add additional agent options after any call specific options
	// NOTE(reed): this policy is open to being the opposite way around, have at it,
	// the thinking being things like FromHTTPRequest are in supplied opts...
	opts = append(opts, a.callOpts...)

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
		ext, err := a.callOverrider(c.req, c.Call, c.extensions)
		if err != nil {
			return nil, err
		}
		c.extensions = ext
	}

	mem := c.Memory + uint64(c.TmpFsSize)
	if !a.resources.IsResourcePossible(mem, c.CPUs) {
		return nil, models.ErrCallResourceTooBig
	}

	if c.Call.Config == nil {
		c.Call.Config = make(models.Config)
	}
	c.Call.Config["FN_LISTENER"] = "unix:" + filepath.Join(iofsDockerMountDest, udsFilename)
	c.Call.Config["FN_FORMAT"] = "http-stream" // TODO: remove this after fdk's forget what it means
	// TODO we could set type here too, for now, or anything else not based in fn/app/trigger config

	setupCtx(&c)

	c.ct = a
	if c.stderr == nil {
		// TODO(reed): is line writer is vulnerable to attack?
		// XXX(reed): forcing this as default is not great / configuring it isn't great either. reconsider.
		c.stderr = setupLogger(c.req.Context(), a.cfg.MaxLogSize, !a.cfg.DisableDebugUserLogs, c.Call)
	}
	if c.respWriter == nil {
		// send function output to logs if no writer given (TODO no longer need w/o async?)
		// TODO we could/should probably make this explicit to GetCall, ala 'WithLogger', but it's dupe code (who cares?)
		c.respWriter = c.stderr
	}

	return &c, nil
}

func setupCtx(c *call) {
	ctx, _ := common.LoggerWithFields(c.req.Context(),
		logrus.Fields{"call_id": c.ID, "app_id": c.AppID, "fn_id": c.FnID})
	c.req = c.req.WithContext(ctx)
}

type call struct {
	*models.Call

	respWriter   io.Writer
	req          *http.Request
	stderr       io.ReadWriteCloser
	ct           callTrigger
	slots        *slotQueue
	requestState RequestState
	slotHashId   string
	disableNet   bool
	dockerAuth   docker.Auther // pull config function

	// amount of time attributed to user-code execution
	userExecTime *time.Duration

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
	return c.respWriter.(http.ResponseWriter)
}

func (c *call) StdErr() io.ReadWriteCloser {
	return c.stderr
}

func (c *call) AddUserExecutionTime(dur time.Duration) {
	if c.userExecTime == nil {
		c.userExecTime = new(time.Duration)
	}
	*c.userExecTime += dur
}

func (c *call) GetUserExecutionTime() *time.Duration {
	return c.userExecTime
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

	return c.ct.fireBeforeCall(ctx, c.Model())
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
	c.Call.Stats = stats.Decimate(240, c.Call.Stats)

	// NOTE call this after InsertLog or the buffer will get reset
	c.stderr.Close()

	if err := c.ct.fireAfterCall(ctx, c.Model()); err != nil {
		return err
	}
	return errIn // original error, important for use in sync call returns
}

func GetCallLatencies(c *call) (time.Duration, time.Duration) {
	var schedDuration time.Duration
	var execDuration time.Duration

	if c != nil {
		creat := time.Time(c.CreatedAt)
		start := time.Time(c.StartedAt)
		compl := time.Time(c.CompletedAt)

		if !creat.IsZero() && !start.IsZero() {
			if !start.Before(creat) {
				schedDuration = start.Sub(creat)
				if !compl.Before(start) {
					execDuration = compl.Sub(start)
				}
			}
		}
	}
	return schedDuration, execDuration
}
