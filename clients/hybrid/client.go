package hybrid

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
	opentracing "github.com/opentracing/opentracing-go"
)

type Client interface {
	Enqueue(context.Context, *models.Call) error
	Dequeue(context.Context) ([]*models.Call, error)
	Start(context.Context, *models.Call) error
	Finish(context.Context, *models.Call, io.Reader) error

	// TODO we could/should make GetAppAndRoute endpoint? saves a round trip...
	GetApp(ctx context.Context, appName string) (*models.App, error)
	GetRoute(ctx context.Context, appName, route string) (*models.Route, error)
}

var _ Client = new(client)

type client struct {
	base string
	http *http.Client
}

func New(u string) (Client, error) {
	uri, err := url.Parse(u)
	if err != nil {
		return nil, err
	}

	if uri.Host == "" {
		return nil, errors.New("no host specified for client")
	}
	if uri.Scheme == "" {
		uri.Scheme = "http"
	}
	host := uri.Scheme + "://" + uri.Host + "/v1/"

	httpClient := &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			Dial: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
			MaxIdleConns:        512,
			MaxIdleConnsPerHost: 128,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
			TLSClientConfig: &tls.Config{
				ClientSessionCache: tls.NewLRUClientSessionCache(8096),
			},
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	return &client{
		base: host,
		http: httpClient,
	}, nil
}

func (cl *client) Enqueue(ctx context.Context, c *models.Call) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "hybrid_client_enqueue")
	defer span.Finish()

	err := cl.do(ctx, c, nil, "PUT", "runner", "async")
	return err
}

func (cl *client) Dequeue(ctx context.Context) ([]*models.Call, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "hybrid_client_dequeue")
	defer span.Finish()

	var c struct {
		C []*models.Call `json:"calls"`
	}
	err := cl.do(ctx, nil, &c, "GET", "runner", "async")
	return c.C, err
}

func (cl *client) Start(ctx context.Context, c *models.Call) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "hybrid_client_start")
	defer span.Finish()

	err := cl.do(ctx, c, nil, "POST", "runner", "start")
	return err
}

func (cl *client) Finish(ctx context.Context, c *models.Call, r io.Reader) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "hybrid_client_end")
	defer span.Finish()

	var b bytes.Buffer // TODO pool / we should multipart this?
	_, err := io.Copy(&b, r)
	if err != nil {
		return err
	}
	bod := struct {
		C *models.Call `json:"call"`
		L string       `json:"log"`
	}{
		C: c,
		L: b.String(),
	}

	err = cl.do(ctx, bod, nil, "POST", "runner", "finish")
	return err
}

func (cl *client) GetApp(ctx context.Context, appName string) (*models.App, error) {
	var app models.App
	err := cl.do(ctx, nil, &app, "GET", "apps", appName)
	return &app, err
}

func (cl *client) GetRoute(ctx context.Context, appName, route string) (*models.Route, error) {
	var r models.Route
	err := cl.do(ctx, nil, &r, "GET", "apps", appName, "routes", route)
	return &r, err
}

type httpErr struct {
	code int
	error
}

func (cl *client) do(ctx context.Context, request, result interface{}, method string, url ...string) error {
	// TODO determine policy (should we count to infinity?)

	var b common.Backoff
	for i := 0; i < 5; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		}

		// TODO this isn't re-using buffers very efficiently, but retries should be rare...
		err := cl.once(ctx, request, result, method, url...)
		switch err := err.(type) {
		case nil:
			return err
		case httpErr:
			if err.code < 500 {
				return err
			}
			common.Logger(ctx).WithError(err).Error("error from API server, retrying")
			// retry 500s...
		default:
			// this error wasn't from us [most likely], probably a conn refused/timeout, just retry it out
		}

		b.Sleep(ctx)
	}

	return context.DeadlineExceeded // basically, right?
}

func (cl *client) once(ctx context.Context, request, result interface{}, method string, url ...string) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "hybrid_client_http_do")
	defer span.Finish()

	var b bytes.Buffer // TODO pool
	if request != nil {
		err := json.NewEncoder(&b).Encode(request)
		if err != nil {
			return err
		}
	}

	req, err := http.NewRequest(method, cl.url(url...), &b)
	if err != nil {
		return err
	}
	req = req.WithContext(ctx)
	// shove the span headers in so that the server will continue this span
	opentracing.GlobalTracer().Inject(
		span.Context(),
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(req.Header))

	resp, err := cl.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { io.Copy(ioutil.Discard, resp.Body); resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		// one of our errors
		var msg struct {
			Err *struct {
				Msg string `json:"error"`
			} `json:"error"`
		}
		// copy into a buffer in case it wasn't from us
		var b bytes.Buffer
		io.Copy(&b, resp.Body)
		json.Unmarshal(b.Bytes(), &msg)
		if msg.Err != nil {
			return &httpErr{code: resp.StatusCode, error: errors.New(msg.Err.Msg)}
		}
		return &httpErr{code: resp.StatusCode, error: errors.New(b.String())}
	}

	if result != nil {
		err := json.NewDecoder(resp.Body).Decode(result)
		if err != nil {
			return err
		}
	}

	return nil
}

func (cl *client) url(args ...string) string {
	return cl.base + strings.Join(args, "/")
}
