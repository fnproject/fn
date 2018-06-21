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
	"net/http/httputil"
	"os"
	"strings"
	"testing"

	"github.com/fnproject/fdk-go/utils"
	"time"
)

func echoHTTPHandler(_ context.Context, in io.Reader, out io.Writer) {
	io.Copy(out, in)
	WriteStatus(out, http.StatusTeapot+2)
	SetHeader(out, "yo", "dawg")
}

func TestHandler(t *testing.T) {
	inString := "yodawg"
	var in bytes.Buffer
	io.WriteString(&in, inString)

	var out bytes.Buffer
	echoHTTPHandler(utils.BuildCtx(), &in, &out)

	if out.String() != inString {
		t.Fatalf("this was supposed to be easy. strings no matchy: %s got: %s", inString, out.String())
	}
}

func TestDefault(t *testing.T) {
	inString := "yodawg"
	var in bytes.Buffer
	io.WriteString(&in, inString)

	var out bytes.Buffer

	utils.DoDefault(HandlerFunc(echoHTTPHandler), utils.BuildCtx(), &in, &out)

	if out.String() != inString {
		t.Fatalf("strings no matchy: %s got: %s", inString, out.String())
	}
}

func JSONHandler(_ context.Context, in io.Reader, out io.Writer) {
	var person struct {
		Name string `json:"name"`
	}
	json.NewDecoder(in).Decode(&person)
	if person.Name == "" {
		person.Name = "world"
	}

	body := fmt.Sprintf("Hello %s!\n", person.Name)
	SetHeader(out, "Content-Type", "application/json")
	err := json.NewEncoder(out).Encode(body)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
}

func JSONWithStatusCode(_ context.Context, _ io.Reader, out io.Writer) {
	SetHeader(out, "Content-Type", "application/json")
	WriteStatus(out, 201)
}

func TestJSON(t *testing.T) {
	req := &utils.JsonIn{
		CallID:      "someid",
		Body:        `{"name":"john"}`,
		ContentType: "application/json",
		Deadline:    "2018-01-30T16:52:39.786Z",
		Protocol: utils.CallRequestHTTP{
			Type:       "http",
			RequestURL: "http://localhost:8080/r/myapp/yodawg",
			Headers:    http.Header{},
			Method:     "POST",
		},
	}

	var in bytes.Buffer
	err := json.NewEncoder(&in).Encode(req)
	if err != nil {
		t.Fatal("Unable to marshal request")
	}

	var out, buf bytes.Buffer

	err = utils.DoJSONOnce(HandlerFunc(JSONHandler), utils.BuildCtx(), &in, &out, &buf, make(http.Header))
	if err != nil {
		t.Fatal("should not return error", err)
	}

	JSONOut := &utils.JsonOut{}
	err = json.NewDecoder(&out).Decode(JSONOut)

	if err != nil {
		t.Fatal(err.Error())
	}
	if !strings.Contains(JSONOut.Body, "Hello john!") {
		t.Fatalf("Output assertion mismatch. Expected: `Hello john!\n`. Actual: %v", JSONOut.Body)
	}
	if JSONOut.Protocol.StatusCode != 200 {
		t.Fatalf("Response code must equal to 200, but have: %v", JSONOut.Protocol.StatusCode)
	}
}

func TestFailedJSON(t *testing.T) {
	dummyBody := "should fail with this"
	in := strings.NewReader(dummyBody)

	var out, buf bytes.Buffer

	JSONOut := &utils.JsonOut{}
	err := utils.DoJSONOnce(HandlerFunc(JSONHandler), utils.BuildCtx(), in, &out, &buf, make(http.Header))
	if err != nil {
		t.Fatal("should not return error", err)
	}

	err = json.NewDecoder(&out).Decode(JSONOut)
	if err != nil {
		t.Fatal(err.Error())
	}
	if JSONOut.Protocol.StatusCode != 500 {
		t.Fatalf("Response code must equal to 500, but have: %v", JSONOut.Protocol.StatusCode)
	}
}

func TestJSONEOF(t *testing.T) {
	var in, out, buf bytes.Buffer

	err := utils.DoJSONOnce(HandlerFunc(JSONHandler), utils.BuildCtx(), &in, &out, &buf, make(http.Header))
	if err != io.EOF {
		t.Fatal("should return EOF")
	}
}

func TestJSONOverwriteStatusCodeAndHeaders(t *testing.T) {
	var out, buf bytes.Buffer
	req := &utils.JsonIn{
		CallID:      "someid",
		Body:        `{"name":"john"}`,
		ContentType: "application/json",
		Deadline:    "2018-01-30T16:52:39.786Z",
		Protocol: utils.CallRequestHTTP{
			Type:       "json",
			RequestURL: "http://localhost:8080/r/myapp/yodawg",
			Headers:    http.Header{},
			Method:     "POST",
		},
	}

	var in bytes.Buffer
	err := json.NewEncoder(&in).Encode(req)
	if err != nil {
		t.Fatal("Unable to marshal request")
	}

	err = utils.DoJSONOnce(HandlerFunc(JSONWithStatusCode), utils.BuildCtx(), &in, &out, &buf, make(http.Header))
	if err != nil {
		t.Fatal("should not return error", err)
	}

	JSONOut := &utils.JsonOut{}
	err = json.NewDecoder(&out).Decode(JSONOut)
	if err != nil {
		t.Fatal(err.Error())
	}

	if JSONOut.Protocol.StatusCode != 201 {
		t.Fatalf("Response code must equal to 201, but have: %v", JSONOut.Protocol.StatusCode)
	}
	cType := JSONOut.Protocol.Headers.Get("Content-Type")
	if !strings.Contains(cType, "application/json") {
		t.Fatalf("Response content type should be application/json in this test, but have: %v", cType)
	}
}

func TestHTTP(t *testing.T) {
	// simulate fn writing us http requests...

	bodyString := "yodawg"
	in := HTTPreq(t, bodyString)

	var out bytes.Buffer
	ctx := utils.BuildCtx()
	err := utils.DoHTTPOnce(HandlerFunc(echoHTTPHandler), ctx, in, &out, &bytes.Buffer{}, make(http.Header))
	if err != nil {
		t.Fatal("should not return error", err)
	}

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

func TestHTTPEOF(t *testing.T) {
	var in bytes.Buffer
	var out bytes.Buffer
	ctx := utils.BuildCtx()

	err := utils.DoHTTPOnce(HandlerFunc(echoHTTPHandler), ctx, &in, &out, &bytes.Buffer{}, make(http.Header))
	if err != io.EOF {
		t.Fatal("should return EOF")
	}
}

func HTTPreq(t *testing.T, bod string) io.Reader {
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

func setupTestFromRequest(t *testing.T, data interface{}, contentType, nameTest string) {
	req := &utils.CloudEventIn{
		CloudEvent: utils.CloudEvent{
			EventID:          "someid",
			Source:           "fn-api",
			EventType:        "test-type",
			EventTypeVersion: "1.0",
			EventTime:        time.Now(),
			ContentType:      contentType,
			Data:             data,
		},
		Extensions: utils.CloudEventInExtension{
			Deadline: "2018-01-30T16:52:39.786Z",
			Protocol: utils.CallRequestHTTP{
				Type:       "http",
				RequestURL: "http://localhost:8080/r/myapp/yodawg",
				Headers:    http.Header{},
				Method:     "POST",
			},
		},
	}
	var in bytes.Buffer
	err := json.NewEncoder(&in).Encode(req)
	if err != nil {
		t.Fatal("Unable to marshal request")
	}
	t.Log(in.String())
	var out, buf bytes.Buffer

	err = utils.DoCloudEventOnce(HandlerFunc(JSONHandler), utils.BuildCtx(),
		&in, &out, &buf, make(http.Header))
	if err != nil {
		t.Fatal("should not return error", err)
	}

	t.Log(out.String())
	ceOut := &utils.CloudEventOut{}
	err = json.NewDecoder(&out).Decode(ceOut)
	if err != nil {
		t.Fatal(err.Error())
	}

	if ceOut.Extensions.Protocol.StatusCode != 200 {
		t.Fatalf("Response code must equal to 200, but have: %v", ceOut.Extensions.Protocol.StatusCode)
	}

	var respData string
	json.Unmarshal([]byte(ceOut.Data.(string)), &respData)

	if respData != "Hello "+nameTest+"!\n" {
		t.Fatalf("Output assertion mismatch. Expected: `Hello %v!\n`. Actual: %v", nameTest, ceOut.Data)
	}
}

func TestCloudEventWithJSONData(t *testing.T) {
	data := map[string]string{
		"name": "John",
	}
	contentType := "application/json"
	setupTestFromRequest(t, data, contentType, "John")
}

func TestCloudEventWithStringData(t *testing.T) {
	data := `{"name":"John"}`
	contentType := "text/plain"
	setupTestFromRequest(t, data, contentType, "John")
}

func TestCloudEventWithPerfectlyValidJSONValue(t *testing.T) {
	// https://tools.ietf.org/html/rfc7159#section-3
	data := false
	contentType := "application/json"
	setupTestFromRequest(t, data, contentType, "world")
}
