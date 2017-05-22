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
		rng:  rand.New(rand.NewSource(time.Now().Unix())),
		load: make(map[string]*int64),
	}
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

func (ch *consistentHash) setLoad(key string, load int64) {
	ch.loadMu.RLock()
	l, ok := ch.load[key]
	ch.loadMu.RUnlock()
	if ok {
		atomic.StoreInt64(l, load)
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
		var load int64
		ch.loadMu.RLock()
		loadPtr := ch.load[loadKey(n, key)]
		ch.loadMu.RUnlock()
		if loadPtr != nil {
			load = atomic.LoadInt64(loadPtr)
		}

		// TODO flesh out these values. should be wait times.
		// if we send < 50% of traffic off to other nodes when loaded
		// then as function scales nodes will get flooded, need to be careful.
		//
		// back off loaded node/function combos slightly to spread load
		// TODO do we need a kind of ref counter as well so as to send functions
		// to a different node while there's an outstanding call to another?
		if load < 70 {
			return n
		} else if load > 90 {
			if ch.rng.Intn(100) < 60 {
				return n
			}
		} else if load > 70 {
			if ch.rng.Float64() < 80 {
				return n
			}
		}
		// otherwise loop until we find a sufficiently unloaded node or a lucky coin flip
		return ""
	}

	for _, n := range ch.nodes[i:] {
		node := f(n)
		if node != "" {
			return node, nil
		}
	}

	// try the other half of the ring
	for _, n := range ch.nodes[:i] {
		node := f(n)
		if node != "" {
			return node, nil
		}
	}

	return "", ErrNoNodes
}
