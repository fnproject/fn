package common

import (
	"context"
	"math"
	"math/rand"
	"sync"
	"time"
)

type BoxTime struct{}

func (BoxTime) Now() time.Time                         { return time.Now() }
func (BoxTime) Sleep(d time.Duration)                  { time.Sleep(d) }
func (BoxTime) After(d time.Duration) <-chan time.Time { return time.After(d) }

type Backoff int

func (b *Backoff) Sleep(ctx context.Context) {
	const (
		maxexp   = 7
		interval = 25 * time.Millisecond
	)

	rng := defaultRNG
	clock := defaultClock

	// 25-50ms, 50-100ms, 100-200ms, 200-400ms, 400-800ms, 800-1600ms, 1600-3200ms, 3200-6400ms
	d := time.Duration(math.Pow(2, float64(*b))) * interval
	d += (d * time.Duration(rng.Float64()))

	select {
	case <-ctx.Done():
	case <-clock.After(d):
	}

	if *b < maxexp {
		(*b)++
	}
}

var (
	defaultRNG   = NewRNG(time.Now().UnixNano())
	defaultClock = BoxTime{}
)

func NewRNG(seed int64) *rand.Rand {
	return rand.New(&lockedSource{src: rand.NewSource(seed)})
}

// taken from go1.5.1 math/rand/rand.go +233-250
// bla bla if it puts a hole in the earth don't sue them
type lockedSource struct {
	lk  sync.Mutex
	src rand.Source
}

func (r *lockedSource) Int63() (n int64) {
	r.lk.Lock()
	n = r.src.Int63()
	r.lk.Unlock()
	return
}

func (r *lockedSource) Seed(seed int64) {
	r.lk.Lock()
	r.src.Seed(seed)
	r.lk.Unlock()
}
