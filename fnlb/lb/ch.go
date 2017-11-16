package lb

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dchest/siphash"
)

func NewConsistentRouter(conf Config) Router {
	return &chRouter{
		rng:  rand.New(&lockedSource{src: rand.NewSource(time.Now().Unix()).(rand.Source64)}),
		load: make(map[string]*int64),
	}
}

type chRouter struct {
	// XXX (reed): right now this only supports one client basically ;) use some real stat backend
	statsMu sync.Mutex
	stats   []*stat

	loadMu sync.RWMutex
	load   map[string]*int64
	rng    *rand.Rand
}

type stat struct {
	timestamp time.Time
	latency   time.Duration
	node      string
	code      int
	fx        string
	wait      time.Duration
}

func (ch *chRouter) addStat(s *stat) {
	ch.statsMu.Lock()
	// delete last 1 minute of data if nobody is watching
	for i := 0; i < len(ch.stats) && ch.stats[i].timestamp.Before(time.Now().Add(-1*time.Minute)); i++ {
		ch.stats = ch.stats[:i]
	}
	ch.stats = append(ch.stats, s)
	ch.statsMu.Unlock()
}

func (ch *chRouter) getStats() []*stat {
	ch.statsMu.Lock()
	stats := ch.stats
	ch.stats = ch.stats[:0]
	ch.statsMu.Unlock()

	return stats
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

// Route in this form relies on the nodes being in sorted order so
// that the output will be consistent (yes, slightly unfortunate).
func (ch *chRouter) Route(nodes []string, key string) (string, error) {
	// crc not unique enough & sha is too slow, it's 1 import
	sum64 := siphash.Hash(0, 0x4c617279426f6174, []byte(key))

	i := int(jumpConsistentHash(sum64, int32(len(nodes))))
	return ch.besti(key, i, nodes)
}

func (ch *chRouter) InterceptResponse(req *http.Request, resp *http.Response) {
	load, _ := time.ParseDuration(resp.Header.Get("XXX-FXLB-WAIT"))
	// XXX (reed): we should prob clear this from user response?
	// resp.Header.Del("XXX-FXLB-WAIT") // don't show this to user

	// XXX (reed): need to validate these prob
	ch.setLoad(loadKey(req.URL.Host, req.URL.Path), int64(load))

	ch.addStat(&stat{
		timestamp: time.Now(),
		//latency:   latency, // XXX (reed): plumb
		node: req.URL.Host,
		code: resp.StatusCode,
		fx:   req.URL.Path,
		wait: load,
	})
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

func (ch *chRouter) setLoad(key string, load int64) {
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

func loadKey(node, key string) string {
	return node + "\x00" + key
}

func (ch *chRouter) checkLoad(key, n string) bool {
	var load time.Duration
	ch.loadMu.RLock()
	loadPtr := ch.load[loadKey(n, key)]
	ch.loadMu.RUnlock()
	if loadPtr != nil {
		load = time.Duration(atomic.LoadInt64(loadPtr))
	}

	const (
		// TODO we should probably use deltas rather than fixed wait times. for 'cold'
		// functions these could always trigger. i.e. if wait time increased 5x over last
		// 100 data points, point the cannon elsewhere (we'd have to track 2 numbers but meh)
		lowerLat = 500 * time.Millisecond
		upperLat = 2 * time.Second
	)

	// TODO flesh out these values.
	// if we send < 50% of traffic off to other nodes when loaded
	// then as function scales nodes will get flooded, need to be careful.
	//
	// back off loaded node/function combos slightly to spread load
	if load < lowerLat {
		return true
	} else if load > upperLat {
		// really loaded
		if ch.rng.Intn(100) < 10 { // XXX (reed): 10% could be problematic, should sliding scale prob with log(x) ?
			return true
		}
	} else {
		// 10 < x < 40, as load approaches upperLat, x decreases [linearly]
		x := translate(int64(load), int64(lowerLat), int64(upperLat), 10, 40)
		if ch.rng.Intn(100) < x {
			return true
		}
	}

	// return invalid node to try next node
	return false
}

func (ch *chRouter) besti(key string, i int, nodes []string) (string, error) {
	if len(nodes) < 1 {
		// supposed to be caught in grouper, but double check
		return "", ErrNoNodes
	}

	for ; ; i++ {
		// theoretically this could take infinite time, but practically improbable...
		// TODO we need a way to add a node for a given key from down here if a node is overloaded.
		if ch.checkLoad(key, nodes[i]) {
			return nodes[i], nil
		}

		if i == len(nodes)-1 {
			i = -1 // reset i to 0
		}
	}
}

func translate(val, inFrom, inTo, outFrom, outTo int64) int {
	outRange := outTo - outFrom
	inRange := inTo - inFrom
	inVal := val - inFrom
	// we want the number to be lower as intensity increases
	return int(float64(outTo) - (float64(inVal)/float64(inRange))*float64(outRange))
}

func (ch *chRouter) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/1/lb/stats":
			ch.statsGet(w, r)
			return
		case "/1/lb/dash":
			ch.dash(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (ch *chRouter) statsGet(w http.ResponseWriter, r *http.Request) {
	stats := ch.getStats()

	type st struct {
		Timestamp  time.Time `json:"timestamp"`
		Throughput int       `json:"tp"`
		Node       string    `json:"node"`
		Func       string    `json:"func"`
		Wait       float64   `json:"wait"` // seconds
	}
	var sts []st

	// roll up and calculate throughput per second. idk why i hate myself
	aggs := make(map[string][]*stat)
	for _, s := range stats {
		key := s.node + "/" + s.fx
		if t := aggs[key]; len(t) > 0 && t[0].timestamp.Before(s.timestamp.Add(-1*time.Second)) {
			sts = append(sts, st{
				Timestamp:  t[0].timestamp,
				Throughput: len(t),
				Node:       t[0].node,
				Func:       t[0].fx,
				Wait:       avgWait(t),
			})

			aggs[key] = append(aggs[key][:0], s)
		} else {
			aggs[key] = append(aggs[key], s)
		}
	}

	// leftovers
	for _, t := range aggs {
		sts = append(sts, st{
			Timestamp:  t[0].timestamp,
			Throughput: len(t),
			Node:       t[0].node,
			Func:       t[0].fx,
			Wait:       avgWait(t),
		})
	}

	json.NewEncoder(w).Encode(struct {
		Stats []st `json:"stats"`
	}{
		Stats: sts,
	})
}

func avgWait(stats []*stat) float64 {
	var sum time.Duration
	for _, s := range stats {
		sum += s.wait
	}
	return (sum / time.Duration(len(stats))).Seconds()
}

func (ch *chRouter) dash(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(dashPage))
}
