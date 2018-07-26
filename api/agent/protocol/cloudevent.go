package protocol

import (
	"context"
	"encoding/json"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/event"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"io"
	"io/ioutil"
	"time"
)

type cloudEventProtocol struct {
	source          string
	maxResponseSize uint64
	// These are the container input streams, not the input from the request or the output for the response
	in  io.Writer
	out io.Reader
}

func (p *cloudEventProtocol) IsStreamable() bool {
	return true
}

var ErrMissingCallID = errors.New("Missing callID extension")
var ErrMissingDeadline = errors.New("Missing deadline extension")
var ErrMissingResponseContentType = errors.New("Missing response content type")

func (h *cloudEventProtocol) Dispatch(ctx context.Context, evt *event.Event) (*event.Event, error) {
	ctx, span := trace.StartSpan(ctx, "dispatch_cloudevent")
	defer span.End()

	callID, err := evt.GetCallID()
	if err != nil {
		return nil, ErrMissingCallID
	}

	_, err = evt.GetDeadline()
	if err != nil {
		return nil, ErrMissingDeadline
	}
	_, span = trace.StartSpan(ctx, "dispatch_cloudevent_write_request")
	err = json.NewEncoder(h.in).Encode(evt)
	span.End()
	if err != nil {
		return nil, err
	}

	_, span = trace.StartSpan(ctx, "dispatch_cloudevent_read_response")
	var jout event.Event

	clampReader := common.NewClampReadCloser(ioutil.NopCloser(h.out), h.maxResponseSize, ErrContainerResponseTooLarge)
	errCatcher := common.NewErrorCatchingReader(clampReader)
	decoder := json.NewDecoder(errCatcher)
	err = decoder.Decode(&jout)
	span.End()

	if err != nil {
		lastIOError := errCatcher.LastError()

		if lastIOError != nil && lastIOError != io.EOF {
			return nil, errCatcher.LastError()
		}
		return nil, ErrInvalidContentFromContainer
	}
	err = checkExcessData(decoder)
	if err != nil {
		return nil, err
	}

	// content type is mandatory if data is not specified
	if jout.Data != nil && jout.ContentType == "" {
		return nil, ErrMissingResponseContentType
	}

	// Clamp these to fixed values
	jout.Source = h.source
	jout.EventID = callID
	jout.EventTime = common.DateTime(time.Now())
	jout.CloudEventsVersion = event.DefaultCloudEventVersion
	jout.SetCallID(callID)

	return &jout, nil
}
