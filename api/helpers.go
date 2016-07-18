package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"runtime/debug"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"gopkg.in/inconshreveable/log15.v2"

	"github.com/Sirupsen/logrus"
)

func NotFound(w http.ResponseWriter, r *http.Request) {
	SendError(w, http.StatusNotFound, "Not found")
}

func EndpointNotFound(w http.ResponseWriter, r *http.Request) {
	SendError(w, http.StatusNotFound, "Endpoint not found")
}

func InternalError(w http.ResponseWriter, err error) {
	logrus.Error("internal server error response", "err", err, "stack", string(debug.Stack()))
	SendError(w, http.StatusInternalServerError, "internal error")
}

func InternalErrorDetailed(w http.ResponseWriter, r *http.Request, err error) {
	logrus.Error("internal server error response", "err", err, "endpoint", r.URL.String(), "token", GetTokenString(r), "stack", string(debug.Stack()))
	SendError(w, http.StatusInternalServerError, "internal error")
}

type HTTPError interface {
	error
	StatusCode() int
}

type response struct {
	Msg string `json:"msg"`
}

func SendError(w http.ResponseWriter, code int, msg string) {
	logrus.Debug("HTTP error", "status_code", code, "msg", msg)
	resp := response{Msg: msg}
	RespondCode(w, nil, code, &resp)
}

func SendSuccess(w http.ResponseWriter, msg string, params map[string]interface{}) {
	var v interface{}
	if params == nil {
		v = &response{Msg: msg}
	} else {
		v = params
	}
	RespondCode(w, nil, http.StatusOK, v)
}

func Respond(w http.ResponseWriter, r *http.Request, v interface{}) {
	RespondCode(w, r, http.StatusOK, v)
}

func RespondCode(w http.ResponseWriter, r *http.Request, code int, v interface{}) {
	bytes, err := json.Marshal(v)
	if err != nil {
		logrus.Error("error marshalling HTTP response", "value", v, "type", reflect.TypeOf(v), "err", err)
		InternalError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(bytes)))
	w.WriteHeader(code)
	if _, err := w.Write(bytes); err != nil {
		// older callers don't pass a request
		if r != nil {
			logrus.Error("unable to write HTTP response", "err", err)
		} else {
			logrus.Error("unable to write HTTP response", "err", err)
		}
	}
}

// GetTokenString returns the token string from either the Authorization header or
// the oauth query parameter.
func GetTokenString(r *http.Request) string {
	tok, _ := GetTokenStringType(r)
	return tok
}

func GetTokenStringType(r *http.Request) (tok string, jwt bool) {
	tokenStr := r.URL.Query().Get("oauth")
	if tokenStr != "" {
		return tokenStr, false
	}
	tokenStr = r.URL.Query().Get("jwt")
	jwt = tokenStr != ""
	if tokenStr == "" {
		authHeader := r.Header.Get("Authorization")
		authFields := strings.Fields(authHeader)
		if len(authFields) == 2 && (authFields[0] == "OAuth" || authFields[0] == "JWT") {
			jwt = authFields[0] == "JWT"
			tokenStr = authFields[1]
		}
	}
	return tokenStr, jwt
}

func ReadJSONSize(w http.ResponseWriter, r *http.Request, v interface{}, n int64) (success bool) {
	contentType := r.Header.Get("Content-Type")
	i := strings.IndexByte(contentType, ';')
	if i < 0 {
		i = len(contentType)
	}
	if strings.TrimRight(contentType[:i], " ") != "application/json" {
		SendError(w, http.StatusBadRequest, "Bad Content-Type.")
		return false
	}
	if i < len(contentType) {
		param := strings.Trim(contentType[i+1:], " ")
		split := strings.SplitN(param, "=", 2)
		if len(split) != 2 || strings.Trim(split[0], " ") != "charset" {
			SendError(w, http.StatusBadRequest, "Invalid Content-Type parameter.")
			return false
		}
		value := strings.Trim(split[1], " ")
		if len(value) > 2 && value[0] == '"' && value[len(value)-1] == '"' {
			// quoted string
			value = value[1 : len(value)-1]
		}
		if !strings.EqualFold(value, "utf-8") {
			SendError(w, http.StatusBadRequest, "Unsupported charset. JSON is always UTF-8 encoded.")
			return false
		}
	}
	if r.ContentLength > n {
		SendError(w, http.StatusBadRequest, fmt.Sprint("Content-Length greater than", n, "bytes"))
		return false
	}

	err := json.NewDecoder(&LimitedReader{r.Body, n}).Decode(v)
	if err != nil {
		jsonError(w, err)
		return false
	}

	return true
}

func ReadJSON(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	return ReadJSONSize(w, r, v, 100*0xffff)
}

// Same as io.LimitedReader, but returns limitReached so we can
// distinguish between the limit being reached and actual EOF.
type LimitedReader struct {
	R io.Reader
	N int64
}

func (l *LimitedReader) Read(p []byte) (n int, err error) {
	if l.N <= 0 {
		return 0, LimitReached(l.N)
	}
	if int64(len(p)) > l.N {
		p = p[:l.N]
	}
	n, err = l.R.Read(p)
	l.N -= int64(n)
	return
}

// LimitedWriter writes until n bytes are written, then writes
// an overage line and skips any further writes.
type LimitedWriter struct {
	W io.Writer
	N int64
}

func (l *LimitedWriter) Write(p []byte) (n int, err error) {
	var overrage = []byte("maximum log file size exceeded")

	// we expect there may be concurrent writers, so to be safe..
	left := atomic.LoadInt64(&l.N)
	if left <= 0 {
		return 0, io.EOF // TODO EOF? really? does it matter?
	}
	n, err = l.W.Write(p)
	left = atomic.AddInt64(&l.N, -int64(n))
	if left <= 0 {
		l.W.Write(overrage)
	}
	return n, err
}

type JSONError string

func (e JSONError) Error() string { return string(e) }

func jsonType(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
		reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16,
		reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Map, reflect.Struct:
		return "object"
	case reflect.Slice, reflect.Array:
		return "array"
	case reflect.Ptr:
		return jsonType(t.Elem())
	}
	// bool, string, other cases not covered
	return t.String()
}

func jsonError(w http.ResponseWriter, err error) {
	var msg string
	switch err := err.(type) {
	case *json.InvalidUTF8Error:
		msg = "Invalid UTF-8 in JSON: " + err.S

	case *json.InvalidUnmarshalError, *json.UnmarshalFieldError,
		*json.UnsupportedTypeError:
		// should never happen
		InternalError(w, err)
		return

	case *json.SyntaxError:
		msg = fmt.Sprintf("In JSON, %v at position %v.", err, err.Offset)

	case *json.UnmarshalTypeError:
		msg = fmt.Sprintf("In JSON, cannot use %v as %v", err.Value, jsonType(err.Type))

	case *time.ParseError:
		msg = "Time strings must be in RFC 3339 format."

	case LimitReached:
		msg = err.Error()

	case JSONError:
		msg = string(err)

	default:
		if err != io.EOF {
			log15.Error("unhandled json.Unmarshal error", "type", reflect.TypeOf(err), "err", err)
		}
		msg = "Failed to decode JSON."
	}
	SendError(w, http.StatusBadRequest, msg)
}

type sizer interface {
	Size() int64
}

type LimitReached int64

func (e LimitReached) Error() string {
	return fmt.Sprint("Request body greater than", int64(e), "bytes")
}
