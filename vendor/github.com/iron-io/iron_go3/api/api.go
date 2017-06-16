// api provides common functionality for all the iron.io APIs
package api

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/iron-io/iron_go3/config"
)

type DefaultResponseBody struct {
	Msg string `json:"msg"`
}

type URL struct {
	URL         url.URL
	ContentType string
	Settings    config.Settings
}

var (
	Debug            bool
	DebugOnErrors    bool
	DefaultCacheSize = 8192

	// HttpClient is the client used by iron_go to make each http request. It is exported in case
	// the client would like to modify it from the default behavior from http.DefaultClient.
	// This uses the DefaultTransport modified to enable TLS Session Client caching.
	HttpClient = &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			Dial: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
			MaxIdleConnsPerHost: 512,
			TLSHandshakeTimeout: 10 * time.Second,
			TLSClientConfig: &tls.Config{
				ClientSessionCache: tls.NewLRUClientSessionCache(DefaultCacheSize),
			},
		},
	}
)

func dbg(v ...interface{}) {
	if Debug {
		log.Println(v...)
	}
}

func dbgerr(v ...interface{}) {
	if DebugOnErrors && !Debug {
		log.Println(v...)
	}
}

func init() {
	if os.Getenv("IRON_API_DEBUG") != "" {
		Debug = true
		dbg("debugging of api enabled")
	}
	if os.Getenv("IRON_API_DEBUG_ON_ERRORS") != "" {
		DebugOnErrors = true
		dbg("debugging of api on errors enabled")
	}
}

func Action(cs config.Settings, prefix string, suffix ...string) *URL {
	parts := append([]string{prefix}, suffix...)
	return ActionEndpoint(cs, strings.Join(parts, "/"))
}

func RootAction(cs config.Settings, prefix string, suffix ...string) *URL {
	parts := append([]string{prefix}, suffix...)
	return RootActionEndpoint(cs, strings.Join(parts, "/"))
}

func ActionEndpoint(cs config.Settings, endpoint string) *URL {
	u := &URL{Settings: cs, URL: url.URL{}}
	u.URL.Scheme = cs.Scheme
	u.URL.Host = fmt.Sprintf("%s:%d", cs.Host, cs.Port)
	u.URL.Path = fmt.Sprintf("/%s/projects/%s/%s", cs.ApiVersion, cs.ProjectId, endpoint)
	return u
}

func RootActionEndpoint(cs config.Settings, endpoint string) *URL {
	u := &URL{Settings: cs, URL: url.URL{}}
	u.URL.Scheme = cs.Scheme
	u.URL.Host = fmt.Sprintf("%s:%d", cs.Host, cs.Port)
	u.URL.Path = fmt.Sprintf("/%s/%s", cs.ApiVersion, endpoint)
	return u
}

func VersionAction(cs config.Settings) *URL {
	u := &URL{Settings: cs, URL: url.URL{Scheme: cs.Scheme}}
	u.URL.Host = fmt.Sprintf("%s:%d", cs.Host, cs.Port)
	u.URL.Path = "/version"
	return u
}

func (u *URL) QueryAdd(key string, format string, value interface{}) *URL {
	query := u.URL.Query()
	query.Add(key, fmt.Sprintf(format, value))
	u.URL.RawQuery = query.Encode()
	return u
}

func (u *URL) SetContentType(t string) *URL {
	u.ContentType = t
	return u
}

func (u *URL) Req(method string, in, out interface{}) error {
	var body io.ReadSeeker
	switch in := in.(type) {
	case io.ReadSeeker:
		// ready to send (zips uses this)
		body = in
	default:
		if in == nil {
			in = struct{}{}
		}
		data, err := json.Marshal(in)
		if err != nil {
			return err
		}
		dbg("request body:", in)
		body = bytes.NewReader(data)
	}

	response, err := u.req(method, body)
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}

	if err != nil {
		dbg("ERROR!", err, err.Error())
		body := "<empty>"
		if response != nil && response.Body != nil {
			binary, _ := ioutil.ReadAll(response.Body)
			body = string(binary)
		}
		dbgerr("ERROR!", err, err.Error(), "Request:", body, " Response:", body)
		return err
	}
	dbg("response:", response)
	if out != nil {
		return json.NewDecoder(response.Body).Decode(out)
	}

	// throw it away
	io.Copy(ioutil.Discard, response.Body)
	return nil
}

// returned body must be closed by caller if non-nil
func (u *URL) Request(method string, body io.Reader) (response *http.Response, err error) {
	var byts []byte
	if body != nil {
		byts, err = ioutil.ReadAll(body)
		if err != nil {
			return nil, err
		}
	}
	return u.req(method, bytes.NewReader(byts))
}

var MaxRequestRetries = 5

func (u *URL) req(method string, body io.ReadSeeker) (response *http.Response, err error) {
	request, err := http.NewRequest(method, u.URL.String(), nil)
	if err != nil {
		return nil, err
	}

	// body=bytes.Reader implements `Len() int`. if this changes for some reason, looky here
	if s, ok := body.(interface {
		Len() int
	}); ok {
		request.ContentLength = int64(s.Len())
	}
	request.Header.Set("Authorization", "OAuth "+u.Settings.Token)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Accept-Encoding", "gzip/deflate")
	request.Header.Set("User-Agent", u.Settings.UserAgent)

	if u.ContentType != "" {
		request.Header.Set("Content-Type", u.ContentType)
	} else if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	if rc, ok := body.(io.ReadCloser); ok { // stdlib doesn't have ReadSeekCloser :(
		request.Body = rc
	} else {
		request.Body = ioutil.NopCloser(body)
	}

	dbg("URL:", request.URL.String())
	dbg("request:", fmt.Sprintf("%#v\n", request))

	for tries := 0; tries < MaxRequestRetries; tries++ {
		body.Seek(0, 0) // set back to beginning for retries
		response, err = HttpClient.Do(request)
		if err != nil {
			if response != nil && response.Body != nil {
				response.Body.Close() // make sure to close since we won't return it
			}
			if err == io.EOF {
				continue
			}
			return nil, err
		}

		if response.StatusCode == http.StatusServiceUnavailable {
			delay := (tries + 1) * 10 // smooth out delays from 0-2
			time.Sleep(time.Duration(delay*delay) * time.Millisecond)
			continue
		}

		break
	}

	if err != nil { // for that one lucky case where io.EOF reaches MaxRetries
		return nil, err
	}

	if err = ResponseAsError(response); err != nil {
		return nil, err
	}

	return response, nil
}

var HTTPErrorDescriptions = map[int]string{
	http.StatusUnauthorized:     "The OAuth token is either not provided or invalid",
	http.StatusNotFound:         "The resource, project, or endpoint being requested doesn't exist.",
	http.StatusMethodNotAllowed: "This endpoint doesn't support that particular verb",
	http.StatusNotAcceptable:    "Required fields are missing",
}

func ResponseAsError(response *http.Response) HTTPResponseError {
	if response.StatusCode == http.StatusOK || response.StatusCode == http.StatusCreated {
		return nil
	}

	if response == nil {
		return resErr{statusCode: http.StatusTeapot, error: fmt.Sprint("response nil but no errors. beware unicorns, this shouldn't happen")}
	}

	if response.Body != nil {
		defer response.Body.Close()
	}

	var out DefaultResponseBody
	err := json.NewDecoder(response.Body).Decode(&out)
	if err != nil {
		return resErr{statusCode: response.StatusCode, error: fmt.Sprint(response.Status, ": ", err.Error())}
	}
	if out.Msg != "" {
		return resErr{statusCode: response.StatusCode, error: fmt.Sprint(response.Status, ": ", out.Msg)}
	}

	return resErr{statusCode: response.StatusCode, error: response.Status + ": Unknown API Response"}
}

type HTTPResponseError interface {
	Error() string
	StatusCode() int
}

type resErr struct {
	error      string
	statusCode int
}

func (h resErr) Error() string   { return h.error }
func (h resErr) StatusCode() int { return h.statusCode }
