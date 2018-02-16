package agent

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
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
	UpdateState(ctx context.Context, newState ContainerStateType, slots *slotQueue)
}
type RequestState interface {
	UpdateState(ctx context.Context, newState RequestStateType, slots *slotQueue)
}

func NewRequestState() RequestState {
	return &requestState{}
}

func NewContainerState() ContainerState {
	return &containerState{}
}

const (
	RequestStateNone RequestStateType = iota // uninitialized
	RequestStateWait                         // request is waiting
	RequestStateExec                         // request is executing
	RequestStateDone                         // request is done
	RequestStateMax
)

const (
	ContainerStateNone  ContainerStateType = iota // uninitialized
	ContainerStateWait                            // resource (cpu + mem) waiting
	ContainerStateStart                           // launching
	ContainerStateIdle                            // running idle
	ContainerStateBusy                            // running busy
	ContainerStateDone                            // exited/failed/done
	ContainerStateMax
)

var containerGaugeKeys = [ContainerStateMax]string{
	"",
	"container_wait_total",
	"container_start_total",
	"container_idle_total",
	"container_busy_total",
	"container_done_total",
}
var containerTimeKeys = [ContainerStateMax]string{
	"",
	"container_wait_duration_seconds",
	"container_start_duration_seconds",
	"container_idle_duration_seconds",
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

func (c *containerState) UpdateState(ctx context.Context, newState ContainerStateType, slots *slotQueue) {

	var now time.Time
	var oldState ContainerStateType
	var before time.Time

	c.lock.Lock()

	// except for 1) switching back to idle from busy (hot containers) or 2)
	// to waiting from done, otherwise we can only move forward in states
	if c.state < newState ||
		(c.state == ContainerStateBusy && newState == ContainerStateIdle) ||
		(c.state == ContainerStateDone && newState == ContainerStateIdle) {

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
		stats.Record(ctx, stats.FindMeasure(gaugeKey).(*stats.Int64Measure).M(-1))
	}

	timeKey := containerTimeKeys[oldState]
	if timeKey != "" {
		stats.Record(ctx, stats.FindMeasure(timeKey).(*stats.Int64Measure).M(int64(now.Sub(before).Round(time.Millisecond))))
	}

	// update new state stats
	gaugeKey = containerGaugeKeys[newState]
	if gaugeKey != "" {
		stats.Record(ctx, stats.FindMeasure(gaugeKey).(*stats.Int64Measure).M(1))
	}
}

func init() {
	// TODO(reed): do we have to do this? the measurements will be tagged on the context, will they be propagated
	// or we have to white list them in the view for them to show up? test...
	appKey, err := tag.NewKey("fn_appname")
	if err != nil {
		logrus.Fatal(err)
	}
	pathKey, err := tag.NewKey("fn_path")
	if err != nil {
		logrus.Fatal(err)
	}

	for _, key := range containerGaugeKeys {
		if key == "" { // leave nil intentionally, let it panic
			continue
		}
		measure, err := stats.Int64(key, "containers in state "+key, "")
		if err != nil {
			logrus.Fatal(err)
		}
		v, err := view.New(
			key,
			"containers in state "+key,
			[]tag.Key{appKey, pathKey},
			measure,
			view.CountAggregation{},
		)
		if err != nil {
			logrus.Fatalf("cannot create view: %v", err)
		}
		if err := view.Register(v); err != nil {
			logrus.Fatal(err)
		}
	}

	for _, key := range containerTimeKeys {
		if key == "" {
			continue
		}
		measure, err := stats.Int64(key, "time spent in container state "+key, "ms")
		if err != nil {
			logrus.Fatal(err)
		}
		v, err := view.New(
			key,
			"time spent in container state "+key,
			[]tag.Key{appKey, pathKey},
			measure,
			view.DistributionAggregation{},
		)
		if err != nil {
			logrus.Fatalf("cannot create view: %v", err)
		}
		if err := view.Register(v); err != nil {
			logrus.Fatal(err)
		}
	}
}
