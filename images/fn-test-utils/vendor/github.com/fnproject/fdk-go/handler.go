package fdk

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// in case we go over the timeout, need to use a pool since prev buffer may not be freed
var bufPool = &sync.Pool{New: func() interface{} { return new(bytes.Buffer) }}

type httpHandler struct {
	handler Handler
}

func (h *httpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	buf := bufPool.Get().(*bytes.Buffer)
	defer bufPool.Put(buf)

	resp := response{
		Buffer: buf,
		status: 200,
		header: w.Header(),
	}

	ctx, cancel := buildCtx(r.Context(), r)
	defer cancel()

	logFrameHeader(r)

	h.handler.Serve(ctx, r.Body, &resp)

	io.Copy(ioutil.Discard, r.Body) // Ignoring error since r.Body may already be closed
	r.Body.Close()

	if _, ok := GetContext(ctx).(HTTPContext); ok {
		// XXX(reed): could put this in a response writer to clean up? not as easy as it looks (ordering wrt WriteHeader())
		encapHeaders(w.Header())
		// here we set the code in headers, but don't write it to the client writer
		w.Header().Set("Fn-Http-Status", strconv.Itoa(resp.status))
	}
	// NOTE: FDKs don't set call status directly on the response at the moment...

	// XXX(reed): 504 if ctx is past due / handle errors with 5xx? just 200 for now
	// copy response from user back up now with headers in place...
	io.Copy(w, buf)

	// XXX(reed): handle streaming, we have to intercept headers but not necessarily body (ie no buffer)
}

// XXX(reed): we can't use this if we want streaming. just. let. go.
type response struct {
	status int
	header http.Header

	// use bytes.Buffer for io.ReaderFrom / io.WriterTo / et al optimization helper methods
	*bytes.Buffer
}

var _ http.ResponseWriter = new(response)

func (r *response) WriteHeader(code int) { r.status = code }
func (r *response) Header() http.Header  { return r.header }

func buildConfig() map[string]string {
	cfg := make(map[string]string, 16)

	for _, e := range os.Environ() {
		vs := strings.SplitN(e, "=", 2)
		if len(vs) < 2 {
			vs = append(vs, "")
		}
		cfg[vs[0]] = vs[1]
	}
	return cfg
}

// encapHeaders modifies headers in place per http gateway protocol
func encapHeaders(hdr http.Header) {
	for k, vs := range hdr {
		if k == "Content-Type" || strings.HasPrefix(k, "Fn-Http-H-") {
			continue // we've passed this one
		}

		// remove them all to add them all back
		hdr.Del(k)

		// prepend this guy, add it back
		k = "Fn-Http-H-" + k
		hdr[k] = vs
	}
}

func withHTTPContext(ctx context.Context) context.Context {
	rctx, ok := GetContext(ctx).(baseCtx)
	if !ok {
		panic("danger will robinson: only call this method with a base context")
	}

	hdr := rctx.Header()
	hctx := httpCtx{baseCtx: rctx}

	// remove garbage (non-'Fn-Http-H-') headers and fixed http headers on first
	// pass, on 2nd pass we can replace all Fn-Http-H with stripped version and
	// skip all we've done.  this costs 2n time (2 iterations) to keep memory
	// usage flat (in place), we can't in place replace in linear time since go
	// map iteration is not 'stable' and we may hit a key twice in 1 iteration
	// and don't know if it's garbage or not. benchmarks prove it's worth it for all n.
	for k, vs := range hdr {
		switch {
		case k == "Content-Type" || strings.HasPrefix(k, "Fn-Http-H-"): // don't delete
		case k == "Fn-Http-Request-Url":
			hctx.requestURL = vs[0]
			delete(hdr, k)
		case k == "Fn-Http-Method":
			hctx.requestMethod = vs[0]
			delete(hdr, k)
		default:
			delete(hdr, k)
		}
	}

	for k, vs := range hdr {
		switch {
		case strings.HasPrefix(k, "Fn-Http-H-"):
			hdr[strings.TrimPrefix(k, "Fn-Http-H-")] = vs
		default: // we've already stripped / Content-Type
		}
	}

	return WithContext(ctx, hctx)
}

func withBaseContext(ctx context.Context, r *http.Request) (_ context.Context, cancel func()) {
	rctx := baseCtx{
		config: buildConfig(), // from env vars (stinky, but effective...)
		callID: r.Header.Get("Fn-Call-Id"),
		header: r.Header,
	}

	ctx = WithContext(ctx, rctx)
	deadline := r.Header.Get("Fn-Deadline")
	return ctxWithDeadline(ctx, deadline)
}

func buildCtx(ctx context.Context, r *http.Request) (_ context.Context, cancel func()) {
	ctx, cancel = withBaseContext(ctx, r)

	if GetContext(ctx).Header().Get("Fn-Intent") == "httprequest" {
		ctx = withHTTPContext(ctx)
	}

	return ctx, cancel
}

func startHTTPServer(ctx context.Context, handler Handler, path string) {
	uri, err := url.Parse(path)
	if err != nil {
		log.Fatalln("url parse error: ", path, err)
	}

	if uri.Scheme != "unix" || uri.Path == "" {
		log.Fatalln("url scheme must be unix with a valid path, got: ", uri.String())
	}

	server := http.Server{
		Handler: &httpHandler{
			handler: handler,
		},
	}

	// try to remove pre-existing UDS: ignore errors here
	phonySock := filepath.Join(filepath.Dir(uri.Path), "phony"+filepath.Base(uri.Path))
	if uri.Scheme == "unix" {
		os.Remove(phonySock)
		os.Remove(uri.Path)
	}

	listener, err := net.Listen(uri.Scheme, phonySock)
	if err != nil {
		log.Fatalln("net.Listen error: ", err)
	}

	if uri.Scheme == "unix" {
		sockPerm(phonySock, uri.Path)
	}

	go func() {
		<-ctx.Done()
		server.Shutdown(ctx) // this ctx won't wait for listeners, but alas...
		// XXX(reed): we're supposed to wait before returning from startHTTPServer... lazy for now
	}()

	err = server.Serve(listener)
	if err != nil && err != http.ErrServerClosed {
		log.Fatalln("serve error: ", err)
	}
}

func sockPerm(phonySock, realSock string) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// somehow this is the best way to get a permissioned sock file, don't ask questions, life is sad and meaningless
	err := os.Chmod(phonySock, 0666)
	if err != nil {
		log.Fatalln("error giving sock file a perm", err)
	}

	err = os.Symlink(filepath.Base(phonySock), realSock)
	if err != nil {
		log.Fatalln("error linking fake sock to real sock", err)
	}
}

// If enabled, print the log framing content.
func logFrameHeader(r *http.Request) {
	framer := os.Getenv("FN_LOGFRAME_NAME")
	if framer == "" {
		return
	}
	valueSrc := os.Getenv("FN_LOGFRAME_HDR")
	if valueSrc == "" {
		return
	}
	id := r.Header.Get(valueSrc)
	if id != "" {
		fmt.Fprintf(os.Stderr, "\n%s=%s\n", framer, id)
		fmt.Fprintf(os.Stdout, "\n%s=%s\n", framer, id)
	}
}
