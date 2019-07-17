package fdk

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// Handler is a function handler, representing 1 invocation of a function
type Handler interface {
	// Serve contains a context with request configuration, the body of the
	// request as a stream of bytes, and a writer to output to; user's may set
	// headers via the resp writer using the fdk's SetHeader/AddHeader methods -
	// if you've a better idea, pipe up.
	Serve(ctx context.Context, body io.Reader, resp io.Writer)
}

// HandlerFunc makes a Handler so that you don't have to!
type HandlerFunc func(ctx context.Context, in io.Reader, out io.Writer)

// Serve implements Handler
func (f HandlerFunc) Serve(ctx context.Context, in io.Reader, out io.Writer) {
	f(ctx, in, out)
}

// HTTPHandler makes a Handler from an http.Handler, if the function invocation
// is from an http trigger the request is identical to the client request to the
// http gateway (sans some hop headers).
func HTTPHandler(h http.Handler) Handler {
	return &httpHandlerFunc{h}
}

type httpHandlerFunc struct {
	http.Handler
}

// Serve implements Handler
func (f *httpHandlerFunc) Serve(ctx context.Context, in io.Reader, out io.Writer) {
	reqURL := "http://localhost/invoke"
	reqMethod := "POST"
	if ctx, ok := GetContext(ctx).(HTTPContext); ok {
		reqURL = ctx.RequestURL()
		reqMethod = ctx.RequestMethod()
	}

	req, err := http.NewRequest(reqMethod, reqURL, in)
	if err != nil {
		panic("cannot re-create request from context")
	}

	req.Header = GetContext(ctx).Header()
	req = req.WithContext(ctx)

	rw, ok := out.(http.ResponseWriter)
	if !ok {
		panic("output is not a response writer, this was poorly planned please yell at me")
	}

	f.ServeHTTP(rw, req)
}

// GetContext will return an fdk Context that can be used to read configuration and
// request information from an incoming request.
func GetContext(ctx context.Context) Context {
	return ctx.Value(ctxKey).(Context)
}

// WithContext adds an fn context to a context context. It is unclear why this is
// an exported method but hey here ya go don't hurt yourself.
func WithContext(ctx context.Context, fnctx Context) context.Context {
	return context.WithValue(ctx, ctxKey, fnctx)
}

type key struct{}

var ctxKey = new(key)

// Context contains all configuration for a function invocation
type Context interface {
	// Config is a map of all env vars set on a function, the base set of fn
	// headers in addition to any app and function configuration
	Config() map[string]string

	// Header are the headers sent to this function invocation
	Header() http.Header

	// ContentType is Header().Get("Content-Type") but with 15 less chars, you are welcome
	ContentType() string

	// CallID is the call id for this function invocation
	CallID() string

	// AppName is Config()["FN_APP_ID"]
	AppID() string

	// FnID is Config()["FN_FN_ID"]
	FnID() string
}

// HTTPContext contains all configuration for a function invocation sourced
// from an http gateway trigger, which will make the function appear to receive
// from the client request they were sourced from, with no additional headers.
type HTTPContext interface {
	Context

	// RequestURL is the request url from the gateway client http request
	RequestURL() string

	// RequestMethod is the request method from the gateway client http request
	RequestMethod() string
}

type baseCtx struct {
	header http.Header
	config map[string]string
	callID string
}

type httpCtx struct {
	// XXX(reed): if we embed we won't preserve the original headers. since we have an
	// interface handy now we could change this under the covers when/if we want... idk
	baseCtx
	requestURL    string
	requestMethod string
}

func (c baseCtx) Config() map[string]string { return c.config }
func (c baseCtx) Header() http.Header       { return c.header }
func (c baseCtx) ContentType() string       { return c.header.Get("Content-Type") }
func (c baseCtx) CallID() string            { return c.callID }
func (c baseCtx) AppID() string             { return c.config["FN_APP_ID"] }
func (c baseCtx) FnID() string              { return c.config["FN_FN_ID"] }

func (c httpCtx) RequestURL() string    { return c.requestURL }
func (c httpCtx) RequestMethod() string { return c.requestMethod }

func ctxWithDeadline(ctx context.Context, fnDeadline string) (context.Context, context.CancelFunc) {
	t, err := time.Parse(time.RFC3339, fnDeadline)
	if err == nil {
		return context.WithDeadline(ctx, t)
	}
	return context.WithCancel(ctx)
}

// AddHeader will add a header onto the function response
func AddHeader(out io.Writer, key, value string) {
	if resp, ok := out.(http.ResponseWriter); ok {
		resp.Header().Add(key, value)
	}
}

// SetHeader will set a header on the function response
func SetHeader(out io.Writer, key, value string) {
	if resp, ok := out.(http.ResponseWriter); ok {
		resp.Header().Set(key, value)
	}
}

// WriteStatus will set the status code to return in the function response
func WriteStatus(out io.Writer, status int) {
	if resp, ok := out.(http.ResponseWriter); ok {
		resp.WriteHeader(status)
	}
}

// Handle will run the event loop for a function. Handle should be invoked
// through main() in a user's function and can handle communication between the
// function and fn server via any of the supported formats.
func Handle(handler Handler) {
	HandleContext(context.Background(), handler)
}

// HandleContext works the same as Handle, but takes a context that will
// exit the handler loop when canceled/timed out.
func HandleContext(ctx context.Context, handler Handler) {
	format, _ := os.LookupEnv("FN_FORMAT")
	if format != "" && format != "http-stream" {
		log.Fatal("only http-stream format is supported, please set function.format=http-stream against your fn service")
	}
	path := os.Getenv("FN_LISTENER")
	startHTTPServer(ctx, handler, path)
}
