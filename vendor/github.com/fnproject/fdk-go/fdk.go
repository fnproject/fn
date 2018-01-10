package fdk

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

type Handler interface {
	Serve(ctx context.Context, in io.Reader, out io.Writer)
}

type HandlerFunc func(ctx context.Context, in io.Reader, out io.Writer)

func (f HandlerFunc) Serve(ctx context.Context, in io.Reader, out io.Writer) {
	f(ctx, in, out)
}

// Context will return an *fn.Ctx that can be used to read configuration and
// request information from an incoming request.
func Context(ctx context.Context) *Ctx {
	return ctx.Value(ctxKey).(*Ctx)
}

func WithContext(ctx context.Context, fnctx *Ctx) context.Context {
	return context.WithValue(ctx, ctxKey, fnctx)
}

// Ctx provides access to Config and Headers from fn.
type Ctx struct {
	Header http.Header
	Config map[string]string
}

type key struct{}

var ctxKey = new(key)

// AddHeader will add a header on the function response, for hot function
// formats.
func AddHeader(out io.Writer, key, value string) {
	if resp, ok := out.(*response); ok {
		resp.header.Add(key, value)
	}
}

// SetHeader will set a header on the function response, for hot function
// formats.
func SetHeader(out io.Writer, key, value string) {
	if resp, ok := out.(*response); ok {
		resp.header.Set(key, value)
	}
}

// WriteStatus will set the status code to return in the function response, for
// hot function formats.
func WriteStatus(out io.Writer, status int) {
	if resp, ok := out.(*response); ok {
		resp.status = status
	}
}

// Handle will run the event loop for a function. Handle should be invoked
// through main() in a user's function and can handle communication between the
// function and fn server via any of the supported formats.
func Handle(handler Handler) {
	format, _ := os.LookupEnv("FN_FORMAT")
	do(handler, format, os.Stdin, os.Stdout)
}

func do(handler Handler, format string, in io.Reader, out io.Writer) {
	ctx := buildCtx()
	switch format {
	case "http":
		doHTTP(handler, ctx, in, out)
	case "json":
		doJSON(handler, ctx, in, out)
	case "default":
		doDefault(handler, ctx, in, out)
	default:
		panic("unknown format (fdk-go): " + format)
	}
}

// doDefault only runs once, since it is a 'cold' function
func doDefault(handler Handler, ctx context.Context, in io.Reader, out io.Writer) {
	setHeaders(ctx, buildHeadersFromEnv())

	ctx, cancel := ctxWithDeadline(ctx)
	defer cancel()

	handler.Serve(ctx, in, out)
}

// doHTTP runs a loop, reading http requests from in and writing
// http responses to out
func doHTTP(handler Handler, ctx context.Context, in io.Reader, out io.Writer) {
	var buf bytes.Buffer
	// maps don't get down-sized, so we can reuse this as it's likely that the
	// user sends in the same amount of headers over and over (but still clear
	// b/w runs) -- buf uses same principle
	hdr := make(http.Header)

	for {
		err := doHTTPOnce(handler, ctx, in, out, &buf, hdr)
		if err != nil {
			break
		}
	}
}

func doJSON(handler Handler, ctx context.Context, in io.Reader, out io.Writer) {
	var buf bytes.Buffer
	hdr := make(http.Header)

	for {
		err := doJSONOnce(handler, ctx, in, out, &buf, hdr)
		if err != nil {
			break
		}
	}
}

type callRequestHTTP struct {
	Type       string      `json:"type"`
	RequestURL string      `json:"request_url"`
	Headers    http.Header `json:"headers"`
}

type jsonIn struct {
	Body        string          `json:"body"`
	ContentType string          `json:"content_type"`
	CallID      string          `json:"call_id"`
	Protocol    callRequestHTTP `json:"protocol"`
}

type callResponseHTTP struct {
	StatusCode int         `json:"status_code,omitempty"`
	Headers    http.Header `json:"headers,omitempty"`
}

type jsonOut struct {
	Body        string           `json:"body"`
	ContentType string           `json:"content_type"`
	Protocol    callResponseHTTP `json:"protocol,omitempty"`
}

func doJSONOnce(handler Handler, ctx context.Context, in io.Reader, out io.Writer, buf *bytes.Buffer, hdr http.Header) error {
	buf.Reset()
	resetHeaders(hdr)

	var jsonResponse jsonOut
	var jsonRequest jsonIn

	resp := response{
		Writer: buf,
		status: 200,
		header: hdr,
	}

	err := json.NewDecoder(in).Decode(&jsonRequest)
	if err != nil {
		// stdin now closed
		if err == io.EOF {
			return err
		}
		jsonResponse.Protocol.StatusCode = 500
		jsonResponse.Body = fmt.Sprintf(`{"error": %v}`, err.Error())
	} else {
		setHeaders(ctx, jsonRequest.Protocol.Headers)
		ctx, cancel := ctxWithDeadline(ctx)
		defer cancel()
		handler.Serve(ctx, strings.NewReader(jsonRequest.Body), &resp)
		jsonResponse.Protocol.StatusCode = resp.status
		jsonResponse.Body = buf.String()
		jsonResponse.Protocol.Headers = resp.header
	}

	json.NewEncoder(out).Encode(jsonResponse)
	return nil
}

func ctxWithDeadline(ctx context.Context) (context.Context, context.CancelFunc) {
	fdkCtx := Context(ctx)
	fnDeadline := fdkCtx.Header.Get("FN_DEADLINE") // this is always in headers

	t, err := time.Parse(time.RFC3339, fnDeadline)
	if err == nil {
		return context.WithDeadline(ctx, t)
	}
	return context.WithCancel(ctx)
}

func doHTTPOnce(handler Handler, ctx context.Context, in io.Reader, out io.Writer, buf *bytes.Buffer, hdr http.Header) error {
	buf.Reset()
	resetHeaders(hdr)
	resp := response{
		Writer: buf,
		status: 200,
		header: hdr,
	}

	req, err := http.ReadRequest(bufio.NewReader(in))
	if err != nil {
		// stdin now closed
		if err == io.EOF {
			return err
		}
		// TODO it would be nice if we could let the user format this response to their preferred style..
		resp.status = http.StatusInternalServerError
		io.WriteString(resp, err.Error())
	} else {
		ctx, cancel := ctxWithDeadline(ctx)
		defer cancel()
		setHeaders(ctx, req.Header)
		handler.Serve(ctx, req.Body, &resp)
	}

	hResp := http.Response{
		ProtoMajor:    1,
		ProtoMinor:    1,
		StatusCode:    resp.status,
		Request:       req,
		Body:          ioutil.NopCloser(buf),
		ContentLength: int64(buf.Len()),
		Header:        resp.header,
	}
	hResp.Write(out)
	return nil
}

func resetHeaders(m http.Header) {
	for k := range m { // compiler optimizes this to 1 instruction now
		delete(m, k)
	}
}

// response is a general purpose response struct any format can use to record
// user's code responses before formatting them appropriately.
type response struct {
	status int
	header http.Header

	io.Writer
}

var (
	base = map[string]struct{}{
		`FN_APP_NAME`: struct{}{},
		`FN_PATH`:     struct{}{},
		`FN_METHOD`:   struct{}{},
		`FN_FORMAT`:   struct{}{},
		`FN_MEMORY`:   struct{}{},
		`FN_TYPE`:     struct{}{},
	}

	headerPre = `FN_HEADER_`

	exact = map[string]struct{}{
		`FN_CALL_ID`:     struct{}{},
		`FN_METHOD`:      struct{}{},
		`FN_REQUEST_URL`: struct{}{},
	}
)

func setHeaders(ctx context.Context, hdr http.Header) {
	fctx := ctx.Value(ctxKey).(*Ctx)
	fctx.Header = hdr
}

func buildCtx() context.Context {
	ctx := &Ctx{
		Config: buildConfig(),
		// allow caller to build headers separately (to avoid map alloc)
	}

	return WithContext(context.Background(), ctx)
}

func buildConfig() map[string]string {
	cfg := make(map[string]string, len(base))

	for _, e := range os.Environ() {
		vs := strings.SplitN(e, "=", 2)
		if len(vs) < 2 {
			vs = append(vs, "")
		}
		cfg[vs[0]] = vs[1]
	}
	return cfg
}

func buildHeadersFromEnv() http.Header {
	env := os.Environ()
	hdr := make(http.Header, len(env)-len(base))

	for _, e := range env {
		vs := strings.SplitN(e, "=", 2)
		hdrKey := headerKey(vs[0])
		if hdrKey == "" {
			continue
		}
		if len(vs) < 2 {
			vs = append(vs, "")
		}
		// rebuild these as 'http' headers
		vs = strings.Split(vs[1], ", ")
		for _, v := range vs {
			hdr.Add(hdrKey, v)
		}
	}
	return hdr
}

// for getting headers out of env
func headerKey(key string) string {
	if strings.HasPrefix(key, headerPre) {
		return strings.TrimPrefix(key, headerPre)
	}
	_, ok := exact[key]
	if ok {
		return key
	}
	return ""
}
