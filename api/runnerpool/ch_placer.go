/* The consistent hash ring from the original fnlb.
   The behaviour of this depends on changes to the runner list leaving it relatively stable.
*/
package runnerpool

import (
	"context"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dchest/siphash"
	"github.com/fnproject/fn/api/models"
	"github.com/sirupsen/logrus"
)

type chPlacer struct {
	cfg PlacerConfig

	load   map[string]*int32
	loadMu sync.RWMutex
	rng    *rand.Rand
}

func NewCHPlacer(cfg *PlacerConfig) Placer {
	logrus.Infof("Creating new CH runnerpool placer with config=%+v", cfg)
	return &chPlacer{
		cfg:  *cfg,
		rng:  rand.New(&lockedSource{src: rand.NewSource(time.Now().Unix()).(rand.Source64)}),
		load: make(map[string]*int32),
	}
}

func (p *chPlacer) GetPlacerConfig() PlacerConfig {
	return p.cfg
}

// This borrows the CH placement algorithm from the original FNLB.
// Because we ask a runner to accept load (queuing on the LB rather than on the nodes), we don't use
// the LB_WAIT to drive placement decisions: runners only accept work if they have the capacity for it.
func (p *chPlacer) PlaceCall(ctx context.Context, rp RunnerPool, call RunnerCall) error {
	state := NewPlacerTracker(ctx, &p.cfg, call)
	defer state.HandleDone()

	key := call.Model().FnID
	sum64 := siphash.Hash(0, 0x4c617279426f6174, []byte(key))

	var runnerPoolErr error
	for {
		var runners []Runner
		runners, runnerPoolErr = rp.Runners(ctx, call)

		i := int(jumpConsistentHash(sum64, int32(len(runners))))
		for j := 0; j < len(runners) && !state.IsDone(); j++ {
			r := runners[i]
			if !p.checkLoad(key, r.Address()) {
				// try to shed load, this simulates a probabilistic coin toss, see checkLoad for details
				// NOTE: this is probabalistic and should converge... it shouldn't take forever, but noting so you know
				continue
			}

			placed, err := state.TryRunner(r, call)

			// set this based on error free placement
			// TODO some errors are irredeemable, I didn't get that far in reasoning but this is ok?
			p.setLoad(key, r.Address(), placed && err == nil)
			if placed {
				return err
			}

			i = (i + 1) % len(runners)
		}

		if !state.RetryAllBackoff(len(runners)) {
			break
		}
	}

	if runnerPoolErr != nil {
		// If we haven't been able to place the function and we got an error
		// from the runner pool, return that error (since we don't have
		// enough runners to handle the current load and the runner pool is
		// having trouble).
		state.HandleFindRunnersFailure(runnerPoolErr)
		return runnerPoolErr
	}
	return models.ErrCallTimeoutServerBusy
}

// A Fast, Minimal Memory, Consistent Hash Algorithm:
// https://arxiv.org/ftp/arxiv/papers/1406/1406.2294.pdf
func jumpConsistentHash(key uint64, num_buckets int32) int32 {
	var b, j int64 = -1, 0
	for j < int64(num_buckets) {
		b = j
		key = key*2862933555777941757 + 1
		j = (b + 1) * int64((1<<31)/(key>>33)+1)
	}
	return int32(b)
}

// tracks last 100 samples... this is the easiest thing to futz with, do it!
const DECAY = 0.01

func ewma(old, new int32) int32 {
	// TODO we're not warming, but that's probably okay for our use case (it is the standard with these)
	return int32((float64(new) * DECAY) + (float64(old) * (1 - DECAY)))
}

func (ch *chPlacer) setLoad(key, node string, load bool) {
	key = loadKey(key, node)

	ch.loadMu.RLock()
	l, ok := ch.load[key]
	ch.loadMu.RUnlock()

	var loadInt int32
	if load {
		// we can just keep the ewma between 0 and 100, if requests succeed we'll stay at 100, and
		// converge back to that point, if they start failing it will trend towards 0. this is easier
		// than loading a float and it's maybe lazy, if you feel inclined, then have at improving this.
		// note that we have to set a floor > 0 to keep functions on a given server, for the sake of
		// something, we can pick 5 (1/20), but it should probably be relative to size of any server.
		loadInt = 100
	}

	if ok {
		// this is a lossy ewma w/ or w/o CAS but if things are moving fast we have plenty of sample
		prev := atomic.LoadInt32(l)
		atomic.StoreInt32(l, ewma(prev, loadInt))
	} else {
		ch.loadMu.Lock()
		if _, ok := ch.load[key]; !ok {
			ch.load[key] = &loadInt
		}
		ch.loadMu.Unlock()
	}
}

func loadKey(key, node string) string {
	return node + "\x00" + key
}

func (ch *chPlacer) checkLoad(key, node string) bool {
	key = loadKey(key, node)

	var load int32
	ch.loadMu.RLock()
	loadPtr := ch.load[key]
	ch.loadMu.RUnlock()
	if loadPtr != nil {
		load = atomic.LoadInt32(loadPtr)
	} else {
		// start off with 100% if we don't know
		load = 100
	}

	// see above: we must set a floor to avoid shutting the valve off to any node completely,
	// when running at capacity we still need to introduce load to any given node, we just want
	// to reduce it significantly for higher % of success. this is naive, but serviceable.
	// this also assumes that if a runner fails health checks it will be removed from
	// the list of runners so we're ignoring the failed runner case here intentionally.
	// this lets 5% of requests in
	if load < 5 {
		// TODO: futz with this, it should be in relation to a server size, probably, too
		load = 5
	}

	// now, we have a probability (load) of success, so get a random number and
	// compare it with our load (high=good chance of success). the theory is that
	// this will still introduce some load to a loaded node, and the load number
	// given enough load should stabilize below 100 and above 0, it's interesting
	// to find what this might actually be under duress (we can optimize then).
	// this will naturally shed load once a machine fills up and once it starts
	// succeeding again it will naturally fill it back up again. play with decay,
	// but we should track function runtimes as well here, really, unless we know
	// capacity and such.
	return ch.rng.Int31n(100) < load
}

type lockedSource struct {
	lk  sync.Mutex
	src rand.Source64
}

func (r *lockedSource) Int63() (n int64) {
	r.lk.Lock()
	n = r.src.Int63()
	r.lk.Unlock()
	return n
}

func (r *lockedSource) Uint64() (n uint64) {
	r.lk.Lock()
	n = r.src.Uint64()
	r.lk.Unlock()
	return n
}

func (r *lockedSource) Seed(seed int64) {
	r.lk.Lock()
	r.src.Seed(seed)
	r.lk.Unlock()
}
