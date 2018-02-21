package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	fdk "github.com/fnproject/fdk-go"
	fdkutils "github.com/fnproject/fdk-go/utils"
)

const (
	InvalidResponseStr = "Olive oil is a liquid fat obtained from olives...\n"
)

type AppRequest struct {
	// if specified we 'sleep' the specified msecs
	SleepTime int `json:"sleepTime,omitempty"`
	// if specified, this is our response http status code
	ResponseCode int `json:"responseCode,omitempty"`
	// if specified, this is our response content-type
	ResponseContentType string `json:"responseContentType,omitempty"`
	// if specified, this is our response content-type.
	// jason doesn't sit with the other kids at school.
	JasonContentType string `json:"jasonContentType,omitempty"`
	// if specified, this is echoed back to client
	EchoContent string `json:"echoContent,omitempty"`
	// verbose mode
	IsDebug bool `json:"isDebug,omitempty"`
	// simulate crash
	IsCrash bool `json:"isCrash,omitempty"`
	// read a file from disk
	ReadFile string `json:"readFile,omitempty"`
	// fill created with with zero bytes of specified size
	ReadFileSize int `json:"readFileSize,omitempty"`
	// create a file on disk
	CreateFile string `json:"createFile,omitempty"`
	// fill created with with zero bytes of specified size
	CreateFileSize int `json:"createFileSize,omitempty"`
	// allocate RAM and hold until next request
	AllocateMemory int `json:"allocateMemory,omitempty"`
	// leak RAM forever
	LeakMemory int `json:"leakMemory,omitempty"`
	// respond with partial output
	ResponseSize int `json:"responseSize,omitempty"`
	// corrupt http or json
	InvalidResponse bool `json:"invalidResponse,omitempty"`
	// TODO: simulate slow read/slow write
	// TODO: simulate partial IO write/read
	// TODO: simulate high cpu usage (async and sync)
	// TODO: simulate large body upload/download
	// TODO: infinite loop
}

// ever growing memory leak chunks
var Leaks []*[]byte

// memory to hold on to at every request, new requests overwrite it.
var Hold []byte

type AppResponse struct {
	Request AppRequest        `json:"request"`
	Headers http.Header       `json:"header"`
	Config  map[string]string `json:"config"`
	Data    map[string]string `json:"data"`
}

func init() {
	Leaks = make([]*[]byte, 0, 0)
}

func getTotalLeaks() int {
	total := 0
	for idx, _ := range Leaks {
		total += len(*(Leaks[idx]))
	}
	return total
}

func AppHandler(ctx context.Context, in io.Reader, out io.Writer) {
	req, resp := processRequest(ctx, in)
	var outto fdkresponse
	outto.Writer = out
	finalizeRequest(&outto, req, resp)
}

func finalizeRequest(out *fdkresponse, req *AppRequest, resp *AppResponse) {
	// custom response code
	if req.ResponseCode != 0 {
		out.Status = req.ResponseCode
	} else {
		out.Status = 200
	}

	// custom content type
	if req.ResponseContentType != "" {
		out.Header.Set("Content-Type", req.ResponseContentType)
	}
	// NOTE: don't add 'application/json' explicitly here as an else,
	// we will test that go's auto-detection logic does not fade since
	// some people are relying on it now

	if req.JasonContentType != "" {
		out.JasonContentType = req.JasonContentType
	}

	json.NewEncoder(out).Encode(resp)
}

func processRequest(ctx context.Context, in io.Reader) (*AppRequest, *AppResponse) {

	fnctx := fdk.Context(ctx)

	var request AppRequest
	json.NewDecoder(in).Decode(&request)

	if request.IsDebug {
		format, _ := os.LookupEnv("FN_FORMAT")
		log.Printf("Received format %v", format)
		log.Printf("Received request %#v", request)
		log.Printf("Received headers %v", fnctx.Header)
		log.Printf("Received config %v", fnctx.Config)
	}

	// simulate load if requested
	if request.SleepTime > 0 {
		if request.IsDebug {
			log.Printf("Sleeping %d", request.SleepTime)
		}
		time.Sleep(time.Duration(request.SleepTime) * time.Millisecond)
	}

	data := make(map[string]string)

	// read a file
	if request.ReadFile != "" {
		if request.IsDebug {
			log.Printf("Reading file %s", request.ReadFile)
		}
		out, err := readFile(request.ReadFile, request.ReadFileSize)
		if err != nil {
			data[request.ReadFile+".read_error"] = err.Error()
		} else {
			data[request.ReadFile+".read_output"] = out
		}
	}

	// create a file
	if request.CreateFile != "" {
		if request.IsDebug {
			log.Printf("Creating file %s (size: %d)", request.CreateFile, request.CreateFileSize)
		}
		err := createFile(request.CreateFile, request.CreateFileSize)
		if err != nil {
			data[request.CreateFile+".create_error"] = err.Error()
		}
	}

	// handle one time alloc request (hold on to the memory until next request)
	if request.AllocateMemory != 0 && request.IsDebug {
		log.Printf("Allocating memory size: %d", request.AllocateMemory)
	}
	Hold = getChunk(request.AllocateMemory)

	// leak memory forever
	if request.LeakMemory != 0 {
		if request.IsDebug {
			log.Printf("Leaking memory size: %d total: %d", request.LeakMemory, getTotalLeaks())
		}
		chunk := getChunk(request.LeakMemory)
		Leaks = append(Leaks, &chunk)
	}

	// simulate crash
	if request.IsCrash {
		panic("Crash requested")
	}

	resp := AppResponse{
		Data:    data,
		Request: request,
		Headers: fnctx.Header,
		Config:  fnctx.Config,
	}

	return &request, &resp
}

func main() {
	format, _ := os.LookupEnv("FN_FORMAT")
	testDo(format, os.Stdin, os.Stdout)
}

func testDo(format string, in io.Reader, out io.Writer) {
	ctx := fdkutils.BuildCtx()
	switch format {
	case "http":
		testDoHTTP(ctx, in, out)
	case "json":
		testDoJSON(ctx, in, out)
	case "default":
		fdkutils.DoDefault(fdk.HandlerFunc(AppHandler), ctx, in, out)
	default:
		panic("unknown format (fdk-go): " + format)
	}
}

// doHTTP runs a loop, reading http requests from in and writing
// http responses to out
func testDoHTTP(ctx context.Context, in io.Reader, out io.Writer) {
	var buf bytes.Buffer
	// maps don't get down-sized, so we can reuse this as it's likely that the
	// user sends in the same amount of headers over and over (but still clear
	// b/w runs) -- buf uses same principle
	hdr := make(http.Header)

	for {
		err := testDoHTTPOnce(ctx, in, out, &buf, hdr)
		if err != nil {
			break
		}
	}
}

func testDoJSON(ctx context.Context, in io.Reader, out io.Writer) {
	var buf bytes.Buffer
	hdr := make(http.Header)

	for {
		err := testDoJSONOnce(ctx, in, out, &buf, hdr)
		if err != nil {
			break
		}
	}
}

func testDoJSONOnce(ctx context.Context, in io.Reader, out io.Writer, buf *bytes.Buffer, hdr http.Header) error {
	buf.Reset()
	fdkutils.ResetHeaders(hdr)
	var resp fdkresponse
	resp.Writer = buf
	resp.Status = 200
	resp.Header = hdr

	responseSize := 0

	var jsonRequest fdkutils.JsonIn
	err := json.NewDecoder(in).Decode(&jsonRequest)
	if err != nil {
		// stdin now closed
		if err == io.EOF {
			return err
		}
		resp.Status = http.StatusInternalServerError
		io.WriteString(resp, fmt.Sprintf(`{"error": %v}`, err.Error()))
	} else {
		fdkutils.SetHeaders(ctx, jsonRequest.Protocol.Headers)
		ctx, cancel := fdkutils.CtxWithDeadline(ctx, jsonRequest.Deadline)
		defer cancel()

		appReq, appResp := processRequest(ctx, strings.NewReader(jsonRequest.Body))
		finalizeRequest(&resp, appReq, appResp)

		if appReq.InvalidResponse {
			io.Copy(out, strings.NewReader(InvalidResponseStr))
		}

		responseSize = appReq.ResponseSize
	}

	jsonResponse := getJSONResp(buf, &resp, &jsonRequest)

	if responseSize > 0 {
		b, err := json.Marshal(jsonResponse)
		if err != nil {
			return err
		}
		if len(b) > responseSize {
			responseSize = len(b)
		}
		b = b[:responseSize]
		out.Write(b)
	} else {
		json.NewEncoder(out).Encode(jsonResponse)
	}

	return nil
}

// since we need to test little jason's content type since he's special. but we
// don't want to add redundant and confusing fields to the fdk...
type fdkresponse struct {
	fdkutils.Response

	JasonContentType string // dumb
}

// copy of fdk.GetJSONResp but with sugar for stupid jason's little fields
func getJSONResp(buf *bytes.Buffer, fnResp *fdkresponse, req *fdkutils.JsonIn) *fdkutils.JsonOut {
	return &fdkutils.JsonOut{
		Body:        buf.String(),
		ContentType: fnResp.JasonContentType,
		Protocol: fdkutils.CallResponseHTTP{
			StatusCode: fnResp.Status,
			Headers:    fnResp.Header,
		},
	}
}

func testDoHTTPOnce(ctx context.Context, in io.Reader, out io.Writer, buf *bytes.Buffer, hdr http.Header) error {
	buf.Reset()
	fdkutils.ResetHeaders(hdr)
	var resp fdkresponse
	resp.Writer = buf
	resp.Status = 200
	resp.Header = hdr

	responseSize := 0

	req, err := http.ReadRequest(bufio.NewReader(in))
	if err != nil {
		// stdin now closed
		if err == io.EOF {
			return err
		}
		// TODO it would be nice if we could let the user format this response to their preferred style..
		resp.Status = http.StatusInternalServerError
		io.WriteString(resp, err.Error())
	} else {
		fnDeadline := fdkutils.Context(ctx).Header.Get("FN_DEADLINE")
		ctx, cancel := fdkutils.CtxWithDeadline(ctx, fnDeadline)
		defer cancel()
		fdkutils.SetHeaders(ctx, req.Header)

		appReq, appResp := processRequest(ctx, req.Body)
		finalizeRequest(&resp, appReq, appResp)

		if appReq.InvalidResponse {
			io.Copy(out, strings.NewReader(InvalidResponseStr))
		}

		responseSize = appReq.ResponseSize
	}

	hResp := fdkutils.GetHTTPResp(buf, &resp.Response, req)

	if responseSize > 0 {

		var buf bytes.Buffer
		bufWriter := bufio.NewWriter(&buf)

		err := hResp.Write(bufWriter)
		if err != nil {
			return err
		}

		bufReader := bufio.NewReader(&buf)

		if buf.Len() > responseSize {
			responseSize = buf.Len()
		}

		_, err = io.CopyN(out, bufReader, int64(responseSize))
		if err != nil {
			return err
		}
	} else {
		hResp.Write(out)
	}

	return nil
}

func getChunk(size int) []byte {
	chunk := make([]byte, size)
	// fill it
	for idx, _ := range chunk {
		chunk[idx] = 1
	}
	return chunk
}

func readFile(name string, size int) (string, error) {
	// read the whole file into memory
	out, err := ioutil.ReadFile(name)
	if err != nil {
		return "", err
	}
	// only respond with partion output if requested
	if size > 0 {
		return string(out[:size]), nil
	}
	return string(out), nil
}

func createFile(name string, size int) error {
	f, err := os.Create(name)
	if err != nil {
		return err
	}

	if size > 0 {
		err := f.Truncate(int64(size))
		if err != nil {
			return err
		}
	}
	return nil
}
