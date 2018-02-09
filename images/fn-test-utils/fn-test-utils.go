package main

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	fdk "github.com/fnproject/fdk-go"
)

type AppRequest struct {
	// if specified we 'sleep' the specified msecs
	SleepTime int `json:"sleepTime,omitempty"`
	// if specified, this is our response http status code
	ResponseCode int `json:"responseCode,omitempty"`
	// if specified, this is our response content-type
	ResponseContentType string `json:"responseContentType,omitempty"`
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
	AllocateMemory int `json:"allocateMemory,om itempty"`
	// leak RAM forever
	LeakMemory int `json:"leakMemory,omitempty"`
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

	fnctx := fdk.Context(ctx)

	var request AppRequest
	json.NewDecoder(in).Decode(&request)

	if request.IsDebug {
		log.Printf("Received request %v", request)
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

	// custom response code
	if request.ResponseCode != 0 {
		fdk.WriteStatus(out, request.ResponseCode)
	} else {
		fdk.WriteStatus(out, 200)
	}

	// custom content type
	if request.ResponseContentType != "" {
		fdk.SetHeader(out, "Content-Type", request.ResponseContentType)
	} else {
		fdk.SetHeader(out, "Content-Type", "application/json")
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

	json.NewEncoder(out).Encode(&resp)
}

func main() {
	fdk.Handle(fdk.HandlerFunc(AppHandler))
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
