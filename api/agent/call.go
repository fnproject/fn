package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.opencensus.io/trace"

	"github.com/fnproject/fn/api/agent/drivers"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
	"github.com/go-openapi/strfmt"
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
type CallOpt func(c *call) error

type Param struct {
	Key   string
	Value string
}
type Params []Param

const (
	ceMimeType = "application/cloudevents+json"
)

// TODO this currently relies on FromRequest having happened before to create the model
// here, to be a fully qualified model. We probably should double check but having a way
// to bypass will likely be what's used anyway unless forced.
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

// TODO this should be required
func WithWriter(w io.Writer) CallOpt {
	return func(c *call) error {
		c.w = w
		return nil
	}
}

func WithContext(ctx context.Context) CallOpt {
	return func(c *call) error {
		c.req = c.req.WithContext(ctx)
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

	if !a.resources.IsResourcePossible(c.Memory, uint64(c.CPUs), c.Type == models.TypeAsync) {
		// if we're not going to be able to run this call on this machine, bail here.
		return nil, models.ErrCallTimeoutServerBusy
	}

	c.da = a.da
	c.ct = a

	ctx, _ := common.LoggerWithFields(c.req.Context(),
		logrus.Fields{"id": c.ID, "app_id": c.AppID, "route": c.Path})
	c.req = c.req.WithContext(ctx)

	// setup stderr logger separate (don't inherit ctx vars)
	logger := logrus.WithFields(logrus.Fields{"user_log": true, "app_id": c.AppID, "path": c.Path, "image": c.Image, "call_id": c.ID})
	c.stderr = setupLogger(logger, a.cfg.MaxLogSize)
	if c.w == nil {
		// send STDOUT to logs if no writer given (async...)
		// TODO we could/should probably make this explicit to GetCall, ala 'WithLogger', but it's dupe code (who cares?)
		c.w = c.stderr
	}

	now := time.Now()
	slotDeadline := now.Add(time.Duration(c.Call.Timeout) * time.Second / 2)
	execDeadline := now.Add(time.Duration(c.Call.Timeout) * time.Second)

	c.slotDeadline = slotDeadline
	c.execDeadline = execDeadline

	return &c, nil
}

type call struct {
	*models.Call

	// IsCloudEvent flag whether this was ingested as a cloud event. This may become the default or only way.
	IsCloudEvent bool `json:"is_cloud_event"`

	da             DataAccess
	w              io.Writer
	req            *http.Request
	stderr         io.ReadWriteCloser
	ct             callTrigger
	slots          *slotQueue
	slotDeadline   time.Time
	lbDeadline     time.Time
	execDeadline   time.Time
	requestState   RequestState
	containerState ContainerState
}

func (c *call) LbDeadline() time.Time {
	return c.lbDeadline
}

func (c *call) RequestBody() io.ReadCloser {
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

	c.StartedAt = strfmt.DateTime(time.Now())
	c.Status = "running"

	// Do not write this header if lb-agent
	if c.lbDeadline.IsZero() {
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
	ctx, span := trace.StartSpan(ctx, "agent_call_end")
	defer span.End()

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

	// NOTE call this after InsertLog or the buffer will get reset
	c.stderr.Close()

	if err := c.ct.fireAfterCall(ctx, c.Model()); err != nil {
		return fmt.Errorf("AfterCall: %v", err)
	}

	return errIn // original error, important for use in sync call returns
}
