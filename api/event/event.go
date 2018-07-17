package event

import (
	"encoding/json"
	"github.com/fnproject/fn/api/common"
	"github.com/pkg/errors"
)

// TODO :  Ideally this would be able to pass an arbitrary byte stream in its body (i.e. not be subject to JSON limitations)   but it's not now
// Currently this only accepts valid JSON bodies, or non-json content that is valid UTF-8
// TODO:

// Event is the official JSON representation of a Event: https://github.com/cloudevents/spec/blob/master/serialization.md
type Event struct {
	CloudEventsVersion string                     `json:"DefaultCloudEventVersion"`
	EventID            string                     `json:"eventID"`
	Source             string                     `json:"source"`
	EventType          string                     `json:"eventType"`
	EventTypeVersion   string                     `json:"eventTypeVersion,omitempty"`
	EventTime          common.DateTime            `json:"eventTime,omitempty"`
	SchemaURL          string                     `json:"schemaURL,omitempty"`
	ContentType        string                     `json:"contentType,omitempty"`
	Extensions         map[string]json.RawMessage `json:"extensions,omitempty"`
	Data               json.RawMessage            `json:"data,omitempty"` // docs: the payload is encoded into a media format which is specified by the contentType attribute (e.g. application/json)
}

// ExtNotFound indicates that a request for an extension that was not present was made s
var ExtNotFound = errors.New("extension not found")

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
	_, val := ce.Extensions[ext]
	return val
}

// ReadExtension Reads an extension into a value , returning an error if the extension could not be read into the body
func (ce *Event) ReadExtension(ext string, val interface{}) error {
	vext, got := ce.Extensions[ext]
	if !got {
		return ExtNotFound
	}
	return json.Unmarshal(vext, val)
}
