package event

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/fnproject/fn/api/common"
	"mime"
	"strings"
)

const DefaultCloudEventVersion = "0.1"

//
//type BodyType int
//
//// TODO Handle body conversion/derivation
//const (
//	BodyTypeNone   = BodyType(iota)
//	BodyTypeJSON   = BodyType(iota)
//	BodyTypeString = BodyType(iota)
//	BodyTypeBinary = BodyType(iota)
//)

// TODO :  Ideally this would be able to pass an arbitrary byte stream in its body (i.e. not be subject to JSON limitations)   but it's not now
// Currently this only accepts valid JSON bodies, or non-json content that is valid UTF-8
// TODO: Would really prefer this to be an interface with stronger correctness by guarantee

// TODO make these  a real type, deal with late-bound serialization
type BodyMessage = json.RawMessage
type ExtensionMessage = json.RawMessage

// Event is the official JSON representation of a Event: https://github.com/cloudevents/spec/blob/master/serialization.md
type Event struct {
	// EventType - typically a dotted reverse domain -based ID (e.g. io.fnproject.ErrorEvent)
	EventType string `json:"eventType"`
	// CloutEventsVersion - version of cloud events spec
	CloudEventsVersion string `json:"cloudEventsVersion"`
	// Source of event - a URI associated with the producer of the event
	Source string `json:"source"`
	// EventID - a unique identifier of the event with respect io its producer
	EventID string `json:"eventID"`
	// EventTime - the time the event occurred at its producer
	EventTime common.DateTime `json:"eventTime,omitempty"`
	// SchemaURL - schema of the data element of this event
	SchemaURL string `json:"schemaURL,omitempty"`
	// ContentType of the data element on this request
	ContentType string `json:"contentType,omitempty"`
	// Extensions are stored in the serialized form and en/re-coded on demand these are assumed to be immutable at the value level
	Extensions map[string]ExtensionMessage `json:"extensions,omitempty"`
	// Data encapsulates the body of the request
	// TODO : we're tied to the JSON encoding here - ideally this is independent of the underlying encoding used (and (e.g.) supports binary!)
	// At the moment we carry this value around as its raw byte encoding - this gives us more constancy in memory behaviour than (e.g.) the default decoding
	Data BodyMessage `json:"data,omitempty"` // docs: the payload is encoded into a media format which is specified by the contentType attribute (e.g. application/json)
}

// This creates a semi-deep clone of the cloud event, assuming that extensions and the raw body are immutable
func (ce *Event) Clone() *Event {
	nce := *ce
	nce.Extensions = make(map[string]json.RawMessage)
	for k, v := range ce.Extensions {
		nce.Extensions[k] = v
	}
	return &nce
}

// SetExtension adds an extension to this event, serializing it to JSON in the process
func (ce *Event) SetExtension(ext string, val interface{}) error {
	vbytes, err := json.Marshal(val)
	if err != nil {
		return err
	}
	if ce.Extensions == nil {
		ce.Extensions = make(map[string]json.RawMessage)
	}
	ce.Extensions[ext] = json.RawMessage(vbytes)
	return nil
}

// HasExtension returns whether this event has a given extension
func (ce *Event) HasExtension(ext string) bool {
	if ce.Extensions == nil {
		return false
	}
	_, val := ce.Extensions[ext]
	return val
}

// ReadExtension Reads an extension into a value , returning an error if the extension could not be read into the body
func (ce *Event) ReadExtension(ext string, val interface{}) error {

	vext, got := ce.Extensions[ext]
	if !got {
		return fmt.Errorf("extension '%s' not found on event", ext)
	}
	err := json.Unmarshal(vext, val)
	if err != nil {
		return fmt.Errorf("extension '%s' could not be extracted correctly: %s", ext, err)
	}
	return nil
}

//IsJSONContentType  determines if we should treat a specific content type as JSON or not
func IsJSONContentType(contentType string) (bool, error) {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false, err
	}
	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json"), nil
}

// BodyAsRawValue returns the body of the event as a raw string - this is only really used to marshal default functions
// If Data is a  string this returns the raw, unescaped Data string, otherwise it returns the native JSON document
func (ce *Event) BodyAsRawValue() ([]byte, error) {

	if ce.Data == nil {
		return []byte{}, nil
	}
	// TODO make data body a concrete type
	// uuuuuggglly

	if ce.Data[0] == '"' {
		var res string
		err := json.Unmarshal(ce.Data, &res)
		if err != nil {
			return nil, fmt.Errorf("failed to read event body as string: %s", err)
		}
		return []byte(res), nil
	}
	// if the body is not a JSON string then we return it's raw JSON encoding
	return ce.Data, nil

}

func JSONBody(body string) (BodyMessage, error) {
	if !json.Valid([]byte(body)) {
		return nil, errors.New("invalid json body")
	}
	return BodyMessage([]byte(body)), nil
}

func MustJSONBody(body string) BodyMessage {
	b, err := JSONBody(body)

	if err != nil {
		panic(err)
	}
	return b
}

func StringBody(body string) (BodyMessage, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return BodyMessage(b), nil
}

func MustStringBody(body string) BodyMessage {
	b, err := StringBody(body)
	if err != nil {
		panic(err.Error())
	}
	return b
}

func JSONExtMessage(val interface{}) (ExtensionMessage, error) {
	bytes, err := json.Marshal(val)
	if err != nil {
		return nil, err
	}
	return ExtensionMessage(bytes), nil
}

func MustJSONExtMessage(val interface{}) ExtensionMessage {
	ext, err := JSONExtMessage(val)
	if err != nil {
		panic(err)
	}
	return ext
}
