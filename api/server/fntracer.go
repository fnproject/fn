package server

import (
	"strings"

	"github.com/opentracing/opentracing-go"
)

// FnTracer is a custom Tracer which wraps another another tracer
// its main purpose is to wrap the underlying Span in a FnSpan,
// which adds some extra behaviour required for sending tracing spans to prometheus
type FnTracer struct {
	opentracing.Tracer
}

// NewFnTracer returns a new FnTracer which wraps the specified Tracer
func NewFnTracer(t opentracing.Tracer) opentracing.Tracer {
	return &FnTracer{t}
}

// FnTracer implements opentracing.Tracer
// Override StartSpan to wrap the returned Span in a FnSpan
func (fnt FnTracer) StartSpan(operationName string, opts ...opentracing.StartSpanOption) opentracing.Span {
	return NewFnSpan(fnt.Tracer.StartSpan(operationName, opts...))
}

// FnSpan is a custom Span that wraps another span
// which adds some extra behaviour required for sending tracing spans to prometheus
type FnSpan struct {
	opentracing.Span
}

// NewFnSpan returns a new FnSpan which wraps the specified Span
func NewFnSpan(s opentracing.Span) opentracing.Span {
	return &FnSpan{s}
}

// FnSpan implements opentracing.Span
func (fns FnSpan) Finish() {
	fns.copyBaggageItemsToTags()
	fns.Span.Finish()
}

// FnSpan implements opentracing.Span
func (fns FnSpan) FinishWithOptions(opts opentracing.FinishOptions) {
	fns.copyBaggageItemsToTags()
	fns.Span.FinishWithOptions(opts)
}

func (fns FnSpan) copyBaggageItemsToTags() {
	// copy baggage items (which are inherited from the parent) with keys starting with "fn" to tags
	// the PrometheusCollector will send these to Prometheus
	// need to do this because the collector can't access baggage items, but it can access tags
	// whereas here we can access the parent's baggage items, but not its tags
	fns.Context().ForeachBaggageItem(func(k, v string) bool {
		if strings.HasPrefix(k, "fn") {
			fns.SetTag(k, v)
		}
		return true
	})
}
