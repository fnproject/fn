package common

import (
	"math"
	"math/rand"
	"sync"
	"time"
)

type BackOffConfig struct {
	MaxRetries uint64 `json:"max_retries"`
	Interval   uint64 `json:"interval_msec"`
	MaxDelay   uint64 `json:"max_delay_msec"`
	MinDelay   uint64 `json:"min_delay_msec"`
}

type BackOff interface {
	NextBackOff() (time.Duration, bool)
}

const RetryForever uint64 = math.MaxUint64

type backoff struct {
	cfg      BackOffConfig
	rand     *rand.Rand
	attempts uint64
	pow      uint64
}

func NewBackOff(cfg BackOffConfig) BackOff {
	// if retries are enabled, then check the config
	if cfg.MaxRetries != 0 {
		if cfg.MaxDelay != 0 && cfg.MinDelay > cfg.MaxDelay {
			panic("invalid max min delay")
		}
	}

	b := &backoff{
		cfg:  cfg,
		rand: rand.New(&lockedSource{src: rand.NewSource(time.Now().UnixNano())}),
		pow:  1,
	}

	return b
}

func (b *backoff) NextBackOff() (time.Duration, bool) {
	// check if retries disabled
	if b.cfg.MaxRetries == 0 {
		return 0, false
	}

	// check max retries if enabled
	if b.cfg.MaxRetries != RetryForever {
		if b.attempts >= b.cfg.MaxRetries {
			return 0, false
		}
		b.attempts += 1
	}

	// https://en.wikipedia.org/wiki/Exponential_backoff

	// 2^c
	if b.pow < math.MaxUint64>>1 {
		b.pow = b.pow * 2
	}

	// (2^c - 1) * slot time
	delay := math.MaxUint64 / b.cfg.Interval
	if delay >= b.pow-1 {
		delay = (b.pow - 1) * b.cfg.Interval
	}

	// check for max
	if b.cfg.MaxDelay != 0 && delay > b.cfg.MaxDelay-b.cfg.MinDelay {
		delay = b.cfg.MaxDelay - b.cfg.MinDelay
	}

	// get rand for max-min
	if delay != 0 {
		delay = b.rand.Uint64() % delay
	} else {
		delay = 0
	}

	// add min
	if math.MaxUint64-b.cfg.MinDelay >= delay {
		delay += b.cfg.MinDelay
	}

	// See if our result overflows time.Duration
	if delay > uint64(math.MaxInt64/time.Millisecond) {
		return time.Duration(math.MaxInt64), true
	}
	return time.Duration(delay) * time.Millisecond, true
}

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
