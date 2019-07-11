package agent

import (
	"context"
	"sync"
	"time"

	"github.com/fnproject/fn/api/agent/drivers"
	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
)

type RequestStateType int
type ContainerStateType int

type containerState struct {
	lock  sync.Mutex
	state ContainerStateType
	start time.Time
}

type requestState struct {
	lock  sync.Mutex
	state RequestStateType
	start time.Time
}

type ContainerState interface {
	UpdateState(ctx context.Context, newState ContainerStateType, call *call)
	GetState() string
}
type RequestState interface {
	UpdateState(ctx context.Context, newState RequestStateType, slots *slotQueue)
}

func NewRequestState() RequestState {
	return &requestState{}
}

func NewContainerState() ContainerState {
	return new(containerState)
}

const (
	RequestStateNone RequestStateType = iota // uninitialized
	RequestStateWait                         // request is waiting
	RequestStateExec                         // request is executing
	RequestStateDone                         // request is done
	RequestStateMax
)

const (
	ContainerStateNone   ContainerStateType = iota // uninitialized
	ContainerStateWait                             // resource (cpu + mem) waiting
	ContainerStateStart                            // launching
	ContainerStateIdle                             // running: idle but not paused
	ContainerStatePaused                           // running: idle but paused
	ContainerStateBusy                             // running: busy
	ContainerStateDone                             // exited/failed/done
	ContainerStateMax
)

var containerStateKeys = [ContainerStateMax]string{
	"none",
	"wait",
	"start",
	"idle",
	"paused",
	"busy",
	"done",
}

var containerGaugeKeys = [ContainerStateMax]string{
	"",
	"container_wait_total",
	"container_start_total",
	"container_idle_total",
	"container_paused_total",
	"container_busy_total",
}

var containerTimeKeys = [ContainerStateMax]string{
	"",
	"container_wait_duration_seconds",
	"container_start_duration_seconds",
	"container_idle_duration_seconds",
	"container_paused_duration_seconds",
	"container_busy_duration_seconds",
}

func (c *requestState) UpdateState(ctx context.Context, newState RequestStateType, slots *slotQueue) {

	var now time.Time
	var oldState RequestStateType

	c.lock.Lock()

	// we can only advance our state forward
	if c.state < newState {

		now = time.Now()
		oldState = c.state
		c.state = newState
		c.start = now
	}

	c.lock.Unlock()

	if now.IsZero() {
		return
	}

	// reflect this change to slot mgr if defined (AKA hot)
	if slots != nil {
		slots.enterRequestState(newState)
		slots.exitRequestState(oldState)
	}
}

func isIdleState(state ContainerStateType) bool {
	return state == ContainerStateIdle || state == ContainerStatePaused
}

func (c *containerState) GetState() string {
	var res ContainerStateType

	c.lock.Lock()
	res = c.state
	c.lock.Unlock()

	return containerStateKeys[res]
}

func (c *containerState) UpdateState(ctx context.Context, newState ContainerStateType, call *call) {
	var slots = call.slots

	var now time.Time
	var oldState ContainerStateType
	var before time.Time

	ctx, _ = tag.New(ctx,
		tag.Upsert(AppIDMetricKey, call.AppID),
		tag.Upsert(FnIDMetricKey, call.FnID),
		tag.Upsert(ImageNameMetricKey, drivers.NormalizeImage(call.Image)),
	)

	c.lock.Lock()

	// Only the following state transitions are allowed:
	// 1) any move forward in states as per ContainerStateType order
	// 2) move back: from paused to idle
	// 3) move back: from busy to idle/paused
	if c.state < newState ||
		(c.state == ContainerStatePaused && newState == ContainerStateIdle) ||
		(c.state == ContainerStateBusy && isIdleState(newState)) {

		now = time.Now()
		oldState = c.state
		before = c.start
		c.state = newState
		c.start = now
	}

	c.lock.Unlock()

	if now.IsZero() {
		return
	}

	// reflect this change to slot mgr if defined (AKA hot)
	if slots != nil {
		slots.enterContainerState(newState)
		slots.exitContainerState(oldState)
	}

	// update old state stats
	gaugeKey := containerGaugeKeys[oldState]
	if gaugeKey != "" {
		stats.Record(ctx, containerGaugeMeasures[oldState].M(-1))
	}

	timeKey := containerTimeKeys[oldState]
	if timeKey != "" {
		stats.Record(ctx, containerTimeMeasures[oldState].M(int64(now.Sub(before)/time.Millisecond)))
	}

	// update new state stats
	gaugeKey = containerGaugeKeys[newState]
	if gaugeKey != "" {
		stats.Record(ctx, containerGaugeMeasures[newState].M(1))
	}
}
