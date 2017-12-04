package lb

import (
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"sync"

	"github.com/coreos/go-semver/semver"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/openzipkin/zipkin-go-opentracing"
	"github.com/sirupsen/logrus"
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
	Driver               string          `json:"driver"`
	DBurl                string          `json:"db_url"`
	Listen               string          `json:"port"`
	MgmtListen           string          `json:"mgmt_port"`
	ShutdownTimeout      int             `json:"shutdown_timeout"`
	ZipkinURL            string          `json:"zipkin_url"`
	Nodes                []string        `json:"nodes"`
	HealthcheckInterval  int             `json:"healthcheck_interval"`
	HealthcheckEndpoint  string          `json:"healthcheck_endpoint"`
	HealthcheckUnhealthy int             `json:"healthcheck_unhealthy"`
	HealthcheckHealthy   int             `json:"healthcheck_healthy"`
	HealthcheckTimeout   int             `json:"healthcheck_timeout"`
	MinAPIVersion        *semver.Version `json:"min_api_version"`

	// Kubernetes support
	Namespace            string          `json:"k8s_namespace"`
	LabelSelector        string          `json:"k8s_label_selector"`

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
	// should not modify the Response as it has already been received nor the
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

	setTracer(conf.ZipkinURL)

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

func setTracer(zipkinURL string) {
	var (
		debugMode          = false
		serviceName        = "fnlb"
		serviceHostPort    = "localhost:8080" // meh
		zipkinHTTPEndpoint = zipkinURL
		// ex: "http://zipkin:9411/api/v1/spans"
	)

	if zipkinHTTPEndpoint == "" {
		return
	}

	logger := zipkintracer.LoggerFunc(func(i ...interface{}) error { logrus.Error(i...); return nil })

	collector, err := zipkintracer.NewHTTPCollector(zipkinHTTPEndpoint, zipkintracer.HTTPLogger(logger))
	if err != nil {
		logrus.WithError(err).Fatalln("couldn't start trace collector")
	}
	tracer, err := zipkintracer.NewTracer(zipkintracer.NewRecorder(collector, debugMode, serviceHostPort, serviceName),
		zipkintracer.ClientServerSameSpan(true),
		zipkintracer.TraceID128Bit(true),
	)
	if err != nil {
		logrus.WithError(err).Fatalln("couldn't start tracer")
	}

	opentracing.SetGlobalTracer(tracer)
	logrus.WithFields(logrus.Fields{"url": zipkinHTTPEndpoint}).Info("started tracer")
}

func (p *proxy) startSpan(req *http.Request) (opentracing.Span, *http.Request) {
	// try to grab a span from the request if made from another service, ignore err if not
	wireContext, _ := opentracing.GlobalTracer().Extract(
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(req.Header))

	// Create the span referring to the RPC client if available.
	// If wireContext == nil, a root span will be created.
	// TODO we should add more tags?
	serverSpan := opentracing.StartSpan("lb_serve", ext.RPCServerOption(wireContext), opentracing.Tag{Key: "path", Value: req.URL.Path})

	ctx := opentracing.ContextWithSpan(req.Context(), serverSpan)
	req = req.WithContext(ctx)
	return serverSpan, req
}

func (p *proxy) RoundTrip(req *http.Request) (*http.Response, error) {
	span, req := p.startSpan(req)
	defer span.Finish()

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

	span, ctx := opentracing.StartSpanFromContext(req.Context(), "lb_roundtrip")
	req = req.WithContext(ctx)

	// shove the span into the outbound request
	opentracing.GlobalTracer().Inject(
		span.Context(),
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(req.Header))

	resp, err := p.transport.RoundTrip(req)
	span.Finish()
	if err == nil {
		p.router.InterceptResponse(req, resp)
	}
	return resp, err
}

func (p *proxy) route(req *http.Request) (string, error) {
	span, ctx := opentracing.StartSpanFromContext(req.Context(), "lb_route")
	defer span.Finish()
	req = req.WithContext(ctx)

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
