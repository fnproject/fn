package lb

import (
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/go-semver/semver"
)

// TODO the load balancers all need to have the same list of nodes. gossip?
// also gossip would handle failure detection instead of elb style. or it can
// be pluggable and then we can read from where bmc is storing them and use that
// or some OSS alternative

// TODO when node goes offline should try to redirect request instead of 5xxing

// TODO we could add some kind of pre-warming call to the functions server where
// the lb could send an image to it to download before the lb starts sending traffic
// there, otherwise when load starts expanding a few functions are going to eat
// the pull time

// TODO config
// TODO TLS

type Config struct {
	DBurl                string          `json:"db_url"`
	Listen               string          `json:"port"`
	Nodes                []string        `json:"nodes"`
	HealthcheckInterval  int             `json:"healthcheck_interval"`
	HealthcheckEndpoint  string          `json:"healthcheck_endpoint"`
	HealthcheckUnhealthy int             `json:"healthcheck_unhealthy"`
	HealthcheckTimeout   int             `json:"healthcheck_timeout"`
	MinAPIVersion        *semver.Version `json:"min_api_version"`

	Transport *http.Transport
}

type Grouper interface {
	// List returns a set of hosts that may be used to route a request
	// for a given key.
	List(key string) ([]string, error)

	// Wrap allows adding middleware to the provided http.Handler.
	Wrap(http.Handler) http.Handler
}

type Router interface {
	// TODO we could probably expose this just as some kind of http.RoundTripper
	// but I can't think of anything elegant so here this is.

	// Route will pick a node from the given set of nodes.
	Route(nodes []string, key string) (string, error)

	// InterceptResponse allows a Router to extract information from proxied
	// requests so that it might do a better job next time. InterceptResponse
	// should not modify the Response as it has already been received nore the
	// Request, having already been sent.
	InterceptResponse(req *http.Request, resp *http.Response)

	// Wrap allows adding middleware to the provided http.Handler.
	Wrap(http.Handler) http.Handler
}

// KeyFunc maps a request to a shard key, it may return an error
// if there are issues locating the shard key.
type KeyFunc func(req *http.Request) (string, error)

type proxy struct {
	keyFunc KeyFunc
	grouper Grouper
	router  Router

	transport http.RoundTripper

	// embed for lazy ServeHTTP mostly
	*httputil.ReverseProxy
}

// NewProxy will marry the given parameters into an able proxy.
func NewProxy(keyFunc KeyFunc, g Grouper, r Router, conf Config) http.Handler {
	p := new(proxy)
	*p = proxy{
		keyFunc:   keyFunc,
		grouper:   g,
		router:    r,
		transport: conf.Transport,
		ReverseProxy: &httputil.ReverseProxy{
			Director:   func(*http.Request) { /* in RoundTrip so we can error out */ },
			Transport:  p,
			BufferPool: newBufferPool(),
		},
	}

	return p
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

func (p *proxy) RoundTrip(req *http.Request) (*http.Response, error) {
	target, err := p.route(req)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{"url": req.URL.Path}).Error("getting index failed")
		if req.Body != nil {
			io.Copy(ioutil.Discard, req.Body)
			req.Body.Close()
		}
		// XXX (reed): if we let the proxy code write the response it will be body-less. ok?
		return nil, ErrNoNodes
	}

	req.URL.Scheme = "http" // XXX (reed): h2 support
	req.URL.Host = target

	resp, err := p.transport.RoundTrip(req)
	if err == nil {
		p.router.InterceptResponse(req, resp)
	}
	return resp, err
}

func (p *proxy) route(req *http.Request) (string, error) {
	// TODO errors from this func likely could return 401 or so instead of 503 always
	key, err := p.keyFunc(req)
	if err != nil {
		return "", err
	}
	list, err := p.grouper.List(key)
	if err != nil {
		return "", err
	}
	return p.router.Route(list, key)
}
