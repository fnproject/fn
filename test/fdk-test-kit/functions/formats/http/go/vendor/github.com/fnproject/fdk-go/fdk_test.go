package fdk

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"strings"
	"testing"
)

func echoHandler(ctx context.Context, in io.Reader, out io.Writer) {
	io.Copy(out, in)
	WriteStatus(out, http.StatusTeapot+2)
	SetHeader(out, "yo", "dawg")
}

func TestHandler(t *testing.T) {
	inString := "yodawg"
	var in bytes.Buffer
	io.WriteString(&in, inString)

	var out bytes.Buffer
	echoHandler(buildCtx(), &in, &out)

	if out.String() != inString {
		t.Fatalf("this was supposed to be easy. strings no matchy: %s got: %s", inString, out.String())
	}
}

func TestDefault(t *testing.T) {
	inString := "yodawg"
	var in bytes.Buffer
	io.WriteString(&in, inString)

	var out bytes.Buffer

	doDefault(HandlerFunc(echoHandler), buildCtx(), &in, &out)

	if out.String() != inString {
		t.Fatalf("strings no matchy: %s got: %s", inString, out.String())
	}
}

func TestHTTP(t *testing.T) {
	// simulate fn writing us http requests...

	bodyString := "yodawg"
	in := req(t, bodyString)

	var out bytes.Buffer
	ctx := buildCtx()
	doHTTPOnce(HandlerFunc(echoHandler), ctx, in, &out, &bytes.Buffer{}, make(http.Header))

	res, err := http.ReadResponse(bufio.NewReader(&out), nil)
	if err != nil {
		t.Fatal(err)
	}

	if res.StatusCode != http.StatusTeapot+2 {
		t.Fatal("got wrong status code", res.StatusCode)
	}

	outBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	if res.Header.Get("yo") != "dawg" {
		t.Fatal("expected yo dawg header, didn't get it")
	}

	if string(outBody) != bodyString {
		t.Fatal("strings no matchy for http", string(outBody), bodyString)
	}
}

func req(t *testing.T, bod string) io.Reader {
	req, err := http.NewRequest("GET", "http://localhost:8080/r/myapp/yodawg", strings.NewReader(bod))
	if err != nil {
		t.Fatal(err)
	}

	byts, err := httputil.DumpRequestOut(req, true)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(byts)
}
