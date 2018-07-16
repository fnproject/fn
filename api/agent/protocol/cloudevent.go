package protocol

import (
	"context"
	"encoding/json"
	"fmt"
	"go.opencensus.io/trace"
	"io"
	"net/http"

	"github.com/fnproject/fn/api/event"
	"github.com/fnproject/fn/api/models"
)

type cloudEventProtocol struct {
	// These are the container input streams, not the input from the request or the output for the response
	in  io.Writer
	out io.Reader
}

func (p *cloudEventProtocol) IsStreamable() bool {
	return true
}

func (h *cloudEventProtocol) writeJSONToContainer(evt *event.Event) error {
	return json.NewEncoder(h.in).Encode(evt)
}

func (h *cloudEventProtocol) Dispatch(ctx context.Context, evt *event.Event) (*event.Event, error) {
	ctx, span := trace.StartSpan(ctx, "dispatch_cloudevent")
	defer span.End()

	_, span = trace.StartSpan(ctx, "dispatch_cloudevent_write_request")
	err := h.writeJSONToContainer(evt)
	span.End()
	if err != nil {
		return nil, err
	}

	_, span = trace.StartSpan(ctx, "dispatch_cloudevent_read_response")
	var jout event.Event

	decoder := json.NewDecoder(h.out)
	err = decoder.Decode(&jout)
	span.End()
	if err != nil {
		return nil, models.NewAPIError(http.StatusBadGateway, fmt.Errorf("invalid json response from function err: %v", err))
	}

	err = checkExcessData(decoder)

	if err != nil {
		return nil, err
	}
	return &jout, nil
}
