package main

import (
	"context"
	"crypto/tls"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
)

type chProxy struct {
	ch *consistentHash

	sync.RWMutex
	// TODO map[string][]time.Time
	ded map[string]int64

	hcInterval  time.Duration
	hcEndpoint  string
	hcUnhealthy int64
	hcTimeout   time.Duration

	// XXX (reed): right now this only supports one client basically ;) use some real stat backend
	statsMu sync.Mutex
	stats   []*stat

	proxy      *httputil.ReverseProxy
	httpClient *http.Client
	transport  http.RoundTripper
}

type stat struct {
	timestamp time.Time
	latency   time.Duration
	node      string
	code      int
	fx        string
	wait      time.Duration
}

func (ch *chProxy) addStat(s *stat) {
	ch.statsMu.Lock()
	// delete last 1 minute of data if nobody is watching
	for i := 0; i < len(ch.stats) && ch.stats[i].timestamp.Before(time.Now().Add(-1*time.Minute)); i++ {
		ch.stats = ch.stats[:i]
	}
	ch.stats = append(ch.stats, s)
	ch.statsMu.Unlock()
}

func (ch *chProxy) getStats() []*stat {
	ch.statsMu.Lock()
	stats := ch.stats
	ch.stats = ch.stats[:0]
	ch.statsMu.Unlock()

	return stats
}

func newProxy(conf config) *chProxy {
	tranny := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 120 * time.Second,
		}).Dial,
		MaxIdleConnsPerHost: 512,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig: &tls.Config{
			ClientSessionCache: tls.NewLRUClientSessionCache(4096),
		},
	}

	ch := &chProxy{
		ded: make(map[string]int64),

		// XXX (reed): need to be reconfigurable at some point
		hcInterval:  time.Duration(conf.HealthcheckInterval) * time.Second,
		hcEndpoint:  conf.HealthcheckEndpoint,
		hcUnhealthy: int64(conf.HealthcheckUnhealthy),
		hcTimeout:   time.Duration(conf.HealthcheckTimeout) * time.Second,
		httpClient:  &http.Client{Transport: tranny},
		transport:   tranny,

		ch: newCH(),
	}

	director := func(req *http.Request) {
		target, err := ch.ch.get(req.URL.Path)
		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{"url": req.URL.Path}).Error("getting index failed")
			target = "error"
		}

		req.URL.Scheme = "http" // XXX (reed): h2 support
		req.URL.Host = target
	}

	ch.proxy = &httputil.ReverseProxy{
		Director:   director,
		Transport:  ch,
		BufferPool: newBufferPool(),
	}

	for _, n := range conf.Nodes {
		ch.ch.add(n)
	}
	go ch.healthcheck()
	return ch
}

type bufferPool struct {
	bufs *sync.Pool
}

func newBufferPool() httputil.BufferPool {
	return &bufferPool{
		bufs: &sync.Pool{
			// 32KB is what the proxy would've used without recycling them
			New: func() interface{} { return make([]byte, 32*1024) },
		},
	}
}

func (b *bufferPool) Get() []byte  { return b.bufs.Get().([]byte) }
func (b *bufferPool) Put(x []byte) { b.bufs.Put(x) }

func (ch *chProxy) RoundTrip(req *http.Request) (*http.Response, error) {
	if req != nil && req.URL.Host == "error" {
		if req.Body != nil {
			io.Copy(ioutil.Discard, req.Body)
			req.Body.Close()
		}
		// XXX (reed): if we let the proxy code write the response it will be body-less. ok?
		return nil, ErrNoNodes
	}

	then := time.Now()
	resp, err := ch.transport.RoundTrip(req)
	if err == nil {
		ch.intercept(req, resp, time.Since(then))
	}
	return resp, err
}

func (ch *chProxy) intercept(req *http.Request, resp *http.Response, latency time.Duration) {
	load, _ := time.ParseDuration(resp.Header.Get("XXX-FXLB-WAIT"))
	// XXX (reed): we should prob clear this from user response?
	// resp.Header.Del("XXX-FXLB-WAIT") // don't show this to user

	// XXX (reed): need to validate these prob
	ch.ch.setLoad(loadKey(req.URL.Host, req.URL.Path), int64(load))

	ch.addStat(&stat{
		timestamp: time.Now(),
		latency:   latency,
		node:      req.URL.Host,
		code:      resp.StatusCode,
		fx:        req.URL.Path,
		wait:      load,
	})
}

func (ch *chProxy) healthcheck() {
	for range time.Tick(ch.hcInterval) {
		nodes := ch.ch.list()
		nodes = append(nodes, ch.dead()...)
		for _, n := range nodes {
			go ch.ping(n)
		}
	}
}

func (ch *chProxy) ping(node string) {
	req, _ := http.NewRequest("GET", "http://"+node+ch.hcEndpoint, nil)
	ctx, cancel := context.WithTimeout(context.Background(), ch.hcTimeout)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := ch.httpClient.Do(req)
	if resp != nil && resp.Body != nil {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}

	if err != nil || resp.StatusCode < 200 || resp.StatusCode > 299 {
		logrus.WithFields(logrus.Fields{"node": node}).Error("health check failed")
		ch.fail(node)
	} else {
		ch.alive(node)
	}
}

func (ch *chProxy) fail(node string) {
	// shouldn't be a hot path so shouldn't be too contended on since health
	// checks are infrequent
	ch.Lock()
	ch.ded[node]++
	failed := ch.ded[node]
	ch.Unlock()

	if failed >= ch.hcUnhealthy {
		ch.ch.remove(node) // TODO under lock?
	}
}

func (ch *chProxy) alive(node string) {
	ch.RLock()
	_, ok := ch.ded[node]
	ch.RUnlock()
	if ok {
		ch.Lock()
		delete(ch.ded, node)
		ch.Unlock()
		ch.ch.add(node) // TODO under lock?
	}
}
