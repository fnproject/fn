package recorder

import (
	"testing"

	"github.com/openzipkin/zipkin-go/model"
)

func TestFlushInRecorderSuccess(t *testing.T) {
	rec := NewReporter()

	span := model.SpanModel{}
	rec.Send(span)

	if len(rec.spans) != 1 {
		t.Fatalf("Span Count want 1, have %d", len(rec.spans))
	}

	rec.Flush()

	if len(rec.spans) != 0 {
		t.Fatalf("Span Count want 0, have %d", len(rec.spans))
	}
}

func TestCloseInRecorderSuccess(t *testing.T) {
	rec := NewReporter()

	span := model.SpanModel{}
	rec.Send(span)

	if len(rec.spans) != 1 {
		t.Fatalf("Span Count want 1, have %d", len(rec.spans))
	}

	rec.Close()

	if len(rec.spans) != 0 {
		t.Fatalf("Span Count want 0, have %d", len(rec.spans))
	}
}
