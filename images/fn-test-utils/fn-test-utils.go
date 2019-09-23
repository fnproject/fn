package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	fdk "github.com/fnproject/fdk-go"
)

const (
	// InvalidResponseStr is a string that isn't one of the 'hot' formats.
	InvalidResponseStr = "Olive oil is a liquid fat obtained from olives...\n"
)

// AppRequest is the body of the input of a function, it can be used to change
// behavior of this function.
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
	// abrupt exit code
	ExitCode int `json:"exitCode,omitempty"`
	// enable abrupt exit
	IsExit bool `json:"isExit,omitempty"`
	// shutdown UDS after request
	IsShutdown bool `json:"isShutdown,omitempty"`
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
	// duplicate trailer if > 0
	TrailerRepeat int `json:"trailerRepeat,omitempty"`
	// corrupt http or json
	InvalidResponse bool `json:"invalidResponse,omitempty"`
	// if specified we 'sleep' the specified msecs *after* processing request
	PostSleepTime int `json:"postSleepTime,omitempty"`
	// spit this out in stdout after processing each request
	PostOutGarbage string `json:"postOutGarbage,omitempty"`
	// spit this out in stderr after processing each request
	PostErrGarbage string `json:"postErrGarbage,omitempty"`
	// test empty body
	IsEmptyBody bool `json:"isEmptyBody,omitempty"`
	// test headers that come into function
	ExpectHeaders map[string][]string `json:"expectHeaders,omitempty"`
	// send some headers out explicitly
	ReturnHeaders map[string][]string `json:"returnHeaders,omitempty"`

	// TODO: simulate slow read/slow write
	// TODO: simulate partial IO write/read
	// TODO: simulate high cpu usage
	// TODO: simulate large body upload/download
	// TODO: infinite loop
}

// Leaks are ever growing memory leak chunks
var Leaks []*[]byte

// Hold is memory to hold on to at every request, new requests overwrite it.
var Hold []byte

var GlobCancel context.CancelFunc

// AppResponse is the output of this function, in JSON
type AppResponse struct {
	Request AppRequest        `json:"request"`
	Headers http.Header       `json:"header"`
	Config  map[string]string `json:"config"`
	Data    map[string]string `json:"data"`
	Trailer []string          `json:"trailer"`
}

func init() {
	Leaks = make([]*[]byte, 0, 0)
}

func getTotalLeaks() int {
	total := 0
	for idx := range Leaks {
		total += len(*(Leaks[idx]))
	}
	return total
}

// AppHandler is the fdk.Handler used by this package
func AppHandler(ctx context.Context, in io.Reader, out io.Writer) {
	req, resp := processRequest(ctx, in)

	if req.InvalidResponse {
		_, err := io.Copy(out, strings.NewReader(InvalidResponseStr))
		if err != nil {
			log.Fatalf("io copy error %v", err)
		}
	}

	finalizeRequest(out, req, resp)
	err := postProcessRequest(req, out)
	if err != nil {
		log.Fatalf("post process error %v", err)
	}
}

func finalizeRequest(out io.Writer, req *AppRequest, resp *AppResponse) {
	// custom content type
	if req.ResponseContentType != "" {
		fdk.SetHeader(out, "Content-Type", req.ResponseContentType)
	}
	// NOTE: don't add 'application/json' explicitly here as an else,
	// we will test that go's auto-detection logic does not fade since
	// some people are relying on it now

	if req.JasonContentType != "" {
		// this will get picked up by our json out handler...
		fdk.SetHeader(out, "Content-Type", req.JasonContentType)
	}

	if req.ReturnHeaders != nil {
		for k, vs := range req.ReturnHeaders {
			for _, v := range vs {
				fdk.AddHeader(out, k, v)
			}
		}
	}

	// custom response code
	if req.ResponseCode != 0 {
		fdk.WriteStatus(out, req.ResponseCode)
	}

	if !req.IsEmptyBody {
		json.NewEncoder(out).Encode(resp)
	}
}

func processRequest(ctx context.Context, in io.Reader) (*AppRequest, *AppResponse) {

	fnctx := fdk.GetContext(ctx)

	var request AppRequest
	json.NewDecoder(in).Decode(&request)

	if request.IsDebug {
		log.Printf("BeginOfLogs")
		log.Printf("Received request %#v", request)
		log.Printf("Received headers %v", fnctx.Header())
		log.Printf("Received config %v", fnctx.Config())
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

	if request.IsDebug {
		info := getDockerInfo()
		log.Printf("DockerInfo %+v", info)
		data["DockerId"] = info.ID
		data["DockerHostname"] = info.Hostname
	}

	// simulate crash
	if request.IsCrash {
		log.Fatalln("Crash requested")
	}

	if request.IsExit {
		log.Printf("Exit requested %+v", request.ExitCode)
		os.Exit(request.ExitCode)
	}

	if request.ExpectHeaders != nil {
		for name, header := range request.ExpectHeaders {
			if h2 := fnctx.Header().Get(name); header[0] != h2 {
				log.Fatalf("Expected header `%s` to be `%s` but was `%s`",
					name, header[0], h2)
			}
		}
	}

	if _, ok := ctx.Deadline(); !ok {
		// XXX(reed): we should plumb the timeout and test it's approximately right but who has time for that?
		log.Fatalf("fdk should set deadline, go fix fdk-go immediately you")
	}

	resp := AppResponse{
		Data:    data,
		Request: request,
		Headers: fnctx.Header(),
		Config:  fnctx.Config(),
		Trailer: make([]string, 0, request.TrailerRepeat),
	}

	for i := request.TrailerRepeat; i > 0; i-- {
		resp.Trailer = append(resp.Trailer, request.EchoContent)
	}

	// Well, almost true.. If panic/errors, we may print stuff after this
	if request.IsDebug {
		log.Printf("EndOfLogs")
	}
	return &request, &resp
}

func postProcessRequest(request *AppRequest, out io.Writer) error {
	if request == nil {
		return nil
	}

	if request.PostSleepTime > 0 {
		if request.IsDebug {
			log.Printf("PostProcess Sleeping %d", request.PostSleepTime)
		}
		time.Sleep(time.Duration(request.PostSleepTime) * time.Millisecond)
	}

	if request.PostOutGarbage != "" {
		if request.IsDebug {
			log.Printf("PostProcess PostOutGarbage %s", request.PostOutGarbage)
		}

		_, err := io.WriteString(out, request.PostOutGarbage)
		if err != nil {
			log.Printf("PostOutGarbage write string error %v", err)
			return err
		}
	}

	if request.PostErrGarbage != "" {
		log.Printf("PostProcess PostErrGarbage %s", request.PostErrGarbage)
	}

	if request.IsShutdown && GlobCancel != nil {
		log.Printf("PostProcess Shutting down UDS")
		GlobCancel()
	}

	return nil
}

func main() {
	if os.Getenv("ENABLE_HEADER") != "" {
		log.Printf("Container starting")
	}

	// simulate long initialization
	if sleeper := os.Getenv("ENABLE_INIT_DELAY_MSEC"); sleeper != "" {
		log.Printf("Container start sleep %v", sleeper)
		delay, err := strconv.ParseInt(sleeper, 10, 64)
		if err != nil {
			log.Fatalf("cannot parse ENABLE_INIT_DELAY_MSEC %v", err)
		}
		time.Sleep(time.Millisecond * time.Duration(delay))
	}

	if initExit := os.Getenv("ENABLE_INIT_EXIT"); initExit != "" {
		log.Printf("Container start exit %v", initExit)
		exitCode, err := strconv.ParseInt(initExit, 10, 64)
		if err != nil {
			log.Fatalf("cannot parse ENABLE_INIT_EXIT %v", err)
		}
		os.Exit(int(exitCode))
	}

	if spawn_match := os.Getenv("ENABLE_FAIL_IF_FN_SPAWN_CALL_ID_NONMATCH"); spawn_match != "" {
		if spawn_id := os.Getenv("FN_SPAWN_CALL_ID"); spawn_id != "" {

			// Non-match case, we need to crash/exit
			if spawn_match != spawn_id {

				sleeper := os.Getenv("ENABLE_FAIL_IF_FN_SPAWN_CALL_ID_NONMATCH_MSEC")

				log.Printf("Container start spawn id non-match our_id(%v) != spawner_id(%v) sleep %v", spawn_id, spawn_match, sleeper)

				// Do we need some sleep?
				if sleeper != "" {
					delay, err := strconv.ParseInt(sleeper, 10, 64)
					if err != nil {
						log.Fatalf("cannot parse ENABLE_FAIL_IF_FN_SPAWN_CALL_ID_NONMATCH_MSEC %v", err)
					}
					time.Sleep(time.Millisecond * time.Duration(delay))
				}
				os.Exit(int(-1))
			}
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	GlobCancel = cancel
	fdk.HandleContext(ctx, fdk.HandlerFunc(AppHandler)) // XXX(reed): can extract & instrument

	// simulate long exit
	if sleeper := os.Getenv("ENABLE_EXIT_DELAY_MSEC"); sleeper != "" {
		log.Printf("Container end sleep %v", sleeper)
		delay, err := strconv.ParseInt(sleeper, 10, 64)
		if err != nil {
			log.Fatalf("cannot parse ENABLE_EXIT_DELAY_MSEC %v", err)
		}
		time.Sleep(time.Millisecond * time.Duration(delay))
	}

	if os.Getenv("ENABLE_FOOTER") != "" {
		log.Printf("Container ending")
	}
}

func getChunk(size int) []byte {
	chunk := make([]byte, size)
	// fill it
	for idx := range chunk {
		chunk[idx] = 1
	}
	return chunk
}

func readFile(name string, size int) (string, error) {
	// read the whole file into memory
	out, err := ioutil.ReadFile(filepath.Clean(name))
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
	f, err := os.Create(filepath.Clean(name))
	if err != nil {
		return err
	}

	if size > 0 {
		// create a 1K block (keep this buffer small to keep
		// memory usage small)
		chunk := make([]byte, 1024)
		for i := 0; i < 1024; i++ {
			chunk[i] = byte(i)
		}

		for size > 0 {
			dlen := size
			if dlen > 1024 {
				dlen = 1024
			}

			_, err := f.Write(chunk[:dlen])
			if err != nil {
				return err
			}

			// slightly modify the chunk to avoid any sparse file possibility
			chunk[0]++
			size = size - dlen
		}
	}
	return nil
}

type dockerInfo struct {
	Hostname string
	ID       string
}

func getDockerInfo() dockerInfo {
	var info dockerInfo

	info.Hostname, _ = os.Hostname()

	// cgroup file has lines such as, where last token is the docker id
	/*
		12:freezer:/docker/610d96c712c6983776f920f2bcf10fae056a6fe5274393c86678ca802d184b0a
	*/
	file, err := os.Open("/proc/self/cgroup")
	if err == nil {
		defer file.Close()
		r := bufio.NewReader(file)
		for {
			line, _, err := r.ReadLine()
			if err != nil {
				break
			}

			tokens := bytes.Split(line, []byte("/"))
			tokLen := len(tokens)
			if tokLen >= 3 && bytes.Compare(tokens[tokLen-2], []byte("docker")) == 0 {
				info.ID = string(tokens[tokLen-1])
				break
			}
		}
	}

	return info
}
