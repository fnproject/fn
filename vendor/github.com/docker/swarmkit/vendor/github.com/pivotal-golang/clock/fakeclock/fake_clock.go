package fakeclock

import (
	"sync"
	"time"

	"github.com/pivotal-golang/clock"
)

type timeWatcher interface {
	timeUpdated(time.Time)
}

type FakeClock struct {
	now time.Time

	watchers map[timeWatcher]struct{}
	cond     *sync.Cond
}

func NewFakeClock(now time.Time) *FakeClock {
	return &FakeClock{
		now:      now,
		watchers: make(map[timeWatcher]struct{}),
		cond:     &sync.Cond{L: &sync.Mutex{}},
	}
}

func (clock *FakeClock) Since(t time.Time) time.Duration {
	return clock.Now().Sub(t)
}

func (clock *FakeClock) Now() time.Time {
	clock.cond.L.Lock()
	defer clock.cond.L.Unlock()

	return clock.now
}

func (clock *FakeClock) Increment(duration time.Duration) {
	clock.increment(duration, false)
}

func (clock *FakeClock) IncrementBySeconds(seconds uint64) {
	clock.Increment(time.Duration(seconds) * time.Second)
}

func (clock *FakeClock) WaitForWatcherAndIncrement(duration time.Duration) {
	clock.increment(duration, true)
}

func (clock *FakeClock) NewTimer(d time.Duration) clock.Timer {
	timer := newFakeTimer(clock, d, false)
	clock.addTimeWatcher(timer)

	return timer
}

func (clock *FakeClock) Sleep(d time.Duration) {
	<-clock.NewTimer(d).C()
}

func (clock *FakeClock) NewTicker(d time.Duration) clock.Ticker {
	timer := newFakeTimer(clock, d, true)
	clock.addTimeWatcher(timer)

	return newFakeTicker(timer)
}

func (clock *FakeClock) WatcherCount() int {
	clock.cond.L.Lock()
	defer clock.cond.L.Unlock()

	return len(clock.watchers)
}

func (clock *FakeClock) increment(duration time.Duration, waitForWatchers bool) {
	clock.cond.L.Lock()

	for waitForWatchers && len(clock.watchers) == 0 {
		clock.cond.Wait()
	}

	now := clock.now.Add(duration)
	clock.now = now

	watchers := make([]timeWatcher, 0, len(clock.watchers))
	for w, _ := range clock.watchers {
		watchers = append(watchers, w)
	}

	clock.cond.L.Unlock()

	for _, w := range watchers {
		w.timeUpdated(now)
	}
}

func (clock *FakeClock) addTimeWatcher(tw timeWatcher) {
	clock.cond.L.Lock()
	clock.watchers[tw] = struct{}{}
	clock.cond.L.Unlock()

	tw.timeUpdated(clock.Now())

	clock.cond.Broadcast()
}

func (clock *FakeClock) removeTimeWatcher(tw timeWatcher) {
	clock.cond.L.Lock()
	delete(clock.watchers, tw)
	clock.cond.L.Unlock()
}
