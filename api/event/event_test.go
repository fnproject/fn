package event

import (
	"encoding/json"
	"reflect"
	"testing"
)

const testStringEvent = `
{
    "cloudEventsVersion" : "0.1",
    "eventType" : "com.example.someevent",
    "source" : "/mycontext",
    "eventID" : "A234-1234-1234",
    "eventTime" : "2018-04-05T17:31:00.000Z",
    "schemaURL" : "http://my-schema.com/schema",
    "extensions" : {
      "comExampleExtension" : "value",
      "objExtesionExt" : {
          "ext": 1,
          "extBool":true
       }
    },
    "contentType" : "text/xml",
    "data" : "<much wow=\"xml\"/>"
}
`
const testJSONEvent = `
{
    "cloudEventsVersion" : "0.1",
    "eventType" : "com.example.someevent",
    "source" : "/mycontext",
    "eventID" : "A234-1234-1234",
    "eventTime" : "2018-04-05T17:31:00.000Z",
    "schemaURL" : "http://my-schema.com/schema",
    "extensions" : {
      "comExampleExtension" : "value"
    },
    "contentType" : "text/xml",
    "data" : {
          "int": 3,
          "array" : [true,false,1.0],
          "sub" : { "a":"b"}
     }
}
`

// round trip the Ce event and verify that the go inerpretation of both input and output are equivalent
func TestCEDeserMatchesOriginal(t *testing.T) {

	var evt Event

	for _, tval := range []string{testJSONEvent, testStringEvent} {
		err := json.Unmarshal([]byte(tval), &evt)
		if err != nil {
			t.Fatalf("error deserializing JSON %s", err)
		}
		v, err := json.Marshal(evt)
		if err != nil {
			t.Fatalf("Failed to marshal evt %s", err)
		}

		var sraw interface{}
		err = json.Unmarshal([]byte(tval), &sraw)
		if err != nil {
			t.Fatal("source raw decode failed")
		}

		var draw interface{}
		err = json.Unmarshal(v, &draw)
		if err != nil {
			t.Fatal("dest raw decode failed")

		}
		if !reflect.DeepEqual(sraw, draw) {
			t.Errorf("re-coded event %s was not equivalent to original event %s", string(v), tval)
		}

	}

}

func TestBodyAsString(t *testing.T) {

	var jsonEvt Event

	tcs := []struct {
		input        string
		expectedBody string
	}{
		{
			input: testJSONEvent,
			expectedBody: `{
          "int": 3,
          "array" : [true,false,1.0],
          "sub" : { "a":"b"}
     }`,
		},
		{
			input:        testStringEvent,
			expectedBody: `<much wow="xml"/>`,
		},
	}

	for _, tc := range tcs {
		err := json.Unmarshal([]byte(tc.input), &jsonEvt)
		if err != nil {
			t.Fatalf("Failed to read event %s", err)
		}

		str, err := jsonEvt.BodyAsRawValue()
		if err != nil {
			t.Fatalf("Failed to read JSON body %s", err)
		}

		if str != tc.expectedBody {
			t.Errorf("invalid Json body; expected '%s', got '%s'", tc.expectedBody, str)
		}
	}

}
