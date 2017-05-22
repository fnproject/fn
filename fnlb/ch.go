package main

import (
	"errors"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dchest/siphash"
)

// consistentHash will maintain a list of strings which can be accessed by
// keying them with a separate group of strings
type consistentHash struct {
	// protects nodes
	sync.RWMutex
	nodes []string

	loadMu sync.RWMutex
	load   map[string]*int64
	rng    *rand.Rand
}

func newCH() *consistentHash {
	return &consistentHash{
		rng:  rand.New(&lockedSource{src: rand.NewSource(time.Now().Unix()).(rand.Source64)}),
		load: make(map[string]*int64),
	}
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

func (ch *consistentHash) add(newb string) {
	ch.Lock()
	defer ch.Unlock()

	// filter dupes, under lock. sorted, so binary search
	i := sort.SearchStrings(ch.nodes, newb)
	if i < len(ch.nodes) && ch.nodes[i] == newb {
		return
	}
	ch.nodes = append(ch.nodes, newb)
	// need to keep in sorted order so that hash index works across nodes
	sort.Sort(sort.StringSlice(ch.nodes))
}

func (ch *consistentHash) remove(ded string) {
	ch.Lock()
	i := sort.SearchStrings(ch.nodes, ded)
	if i < len(ch.nodes) && ch.nodes[i] == ded {
		ch.nodes = append(ch.nodes[:i], ch.nodes[i+1:]...)
	}
	ch.Unlock()
}

// return a copy
func (ch *consistentHash) list() []string {
	ch.RLock()
	ret := make([]string, len(ch.nodes))
	copy(ret, ch.nodes)
	ch.RUnlock()
	return ret
}

func (ch *consistentHash) get(key string) (string, error) {
	// crc not unique enough & sha is too slow, it's 1 import
	sum64 := siphash.Hash(0, 0x4c617279426f6174, []byte(key))

	ch.RLock()
	defer ch.RUnlock()
	i := int(jumpConsistentHash(sum64, int32(len(ch.nodes))))
	return ch.besti(key, i)
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

// tracks last 10 samples (very fast)
const DECAY = 0.1

func ewma(old, new int64) int64 {
	// TODO could 'warm' it up and drop first few samples since we'll have docker pulls / hot starts
	return int64((float64(new) * DECAY) + (float64(old) * (1 - DECAY)))
}

func (ch *consistentHash) setLoad(key string, load int64) {
	ch.loadMu.RLock()
	l, ok := ch.load[key]
	ch.loadMu.RUnlock()
	if ok {
		// this is a lossy ewma w/ or w/o CAS but if things are moving fast we have plenty of sample
		prev := atomic.LoadInt64(l)
		atomic.StoreInt64(l, ewma(prev, load))
	} else {
		ch.loadMu.Lock()
		if _, ok := ch.load[key]; !ok {
			ch.load[key] = &load
		}
		ch.loadMu.Unlock()
	}
}

var (
	ErrNoNodes = errors.New("no nodes available")
)

func loadKey(node, key string) string {
	return node + "\x00" + key
}

// XXX (reed): push down fails / load into ch
func (ch *consistentHash) besti(key string, i int) (string, error) {
	ch.RLock()
	defer ch.RUnlock()

	if len(ch.nodes) < 1 {
		return "", ErrNoNodes
	}

	f := func(n string) string {
		var load time.Duration
		ch.loadMu.RLock()
		loadPtr := ch.load[loadKey(n, key)]
		ch.loadMu.RUnlock()
		if loadPtr != nil {
			load = time.Duration(atomic.LoadInt64(loadPtr))
		}

		const (
			lowerLat = 500 * time.Millisecond
			upperLat = 2 * time.Second
		)

		// TODO flesh out these values.
		// if we send < 50% of traffic off to other nodes when loaded
		// then as function scales nodes will get flooded, need to be careful.
		//
		// back off loaded node/function combos slightly to spread load
		// TODO do we need a kind of ref counter as well so as to send functions
		// to a different node while there's an outstanding call to another?
		if load < lowerLat {
			return n
		} else if load > upperLat {
			// really loaded
			if ch.rng.Intn(100) < 10 { // XXX (reed): 10% could be problematic, should sliding scale prob with log(x) ?
				return n
			}
		} else {
			// 10 < x < 40, as load approaches upperLat, x decreases [linearly]
			x := translate(int64(load), int64(lowerLat), int64(upperLat), 10, 40)
			if ch.rng.Intn(100) < x {
				return n
			}
		}

		// return invalid node to try next node
		return ""
	}

	for ; ; i++ {
		// theoretically this could take infinite time, but practically improbable...
		node := f(ch.nodes[i])
		if node != "" {
			return node, nil
		} else if i == len(ch.nodes)-1 {
			i = -1 // reset i to 0
		}
	}

	panic("strange things are afoot at the circle k")
}

func translate(val, inFrom, inTo, outFrom, outTo int64) int {
	outRange := outTo - outFrom
	inRange := inTo - inFrom
	inVal := val - inFrom
	// we want the number to be lower as intensity increases
	return int(float64(outTo) - (float64(inVal)/float64(inRange))*float64(outRange))
}
