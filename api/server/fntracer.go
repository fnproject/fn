package server

import (
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
	"strings"
)

// FnTracer is a custom Tracer which wraps another another tracer
// its main purpose is to wrap the underlying Span in a FnSpan,
// which adds some extra behaviour required for sending tracing spans to prometheus
type FnTracer struct {
	wrappedTracer opentracing.Tracer
}

// NewFnTracer returns a new FnTracer which wraps the specified Tracer
func NewFnTracer(tracerToWrap opentracing.Tracer) opentracing.Tracer {
	newTracer := &FnTracer{}
	newTracer.wrappedTracer = tracerToWrap
	return newTracer
}

// FnTracer implements opentracing.Tracer
func (thisFnTracer FnTracer) StartSpan(operationName string, opts ...opentracing.StartSpanOption) opentracing.Span {
	return NewFnSpan(thisFnTracer.wrappedTracer.StartSpan(operationName, opts...))
}

// FnTracer implements opentracing.Tracer
func (thisFnTracer FnTracer) Inject(sm opentracing.SpanContext, format interface{}, carrier interface{}) error {
	return thisFnTracer.wrappedTracer.Inject(sm, format, carrier)

}

// FnTracer implements opentracing.Tracer
func (thisFnTracer FnTracer) Extract(format interface{}, carrier interface{}) (opentracing.SpanContext, error) {
	return thisFnTracer.wrappedTracer.Extract(format, carrier)
}

// FnSpan is a custom Span that wraps another span
// which adds some extra behaviour required for sending tracing spans to prometheus
type FnSpan struct {
	wrappedSpan opentracing.Span
}

// NewFnSpan returns a new FnSpan which wraps the specified Span
func NewFnSpan(spanToWrap opentracing.Span) opentracing.Span {
	newSpan := &FnSpan{}
	newSpan.wrappedSpan = spanToWrap
	return newSpan
}

// FnSpan implements opentracing.Span
func (thisFnSpan FnSpan) Finish() {
	thisFnSpan.copyBaggageItemsToTags()
	thisFnSpan.wrappedSpan.Finish()
}

// FnSpan implements opentracing.Span
func (thisFnSpan FnSpan) FinishWithOptions(opts opentracing.FinishOptions) {
	thisFnSpan.copyBaggageItemsToTags()
	thisFnSpan.wrappedSpan.FinishWithOptions(opts)
}

func (thisFnSpan FnSpan) copyBaggageItemsToTags() {
	// copy baggage items (which are inherited from the parent) with keys starting with "fn" to tags
	// the PrometheusCollector will send these to Prometheus
	// need to do this because the collector can't access baggage items, but it can access tags
	// whereas here we can access the parent's baggage items, but not its tags
	thisFnSpan.Context().ForeachBaggageItem(func(k, v string) bool {
		if strings.HasPrefix(k, "fn") {
			thisFnSpan.SetTag(k, v)
		}
		return true
	})
}

// FnSpan implements opentracing.Span
func (thisFnSpan FnSpan) Context() opentracing.SpanContext {
	return thisFnSpan.wrappedSpan.Context()
}

// FnSpan implements opentracing.Span
func (thisFnSpan FnSpan) SetOperationName(operationName string) opentracing.Span {
	return thisFnSpan.wrappedSpan.SetOperationName(operationName)
}

// FnSpan implements opentracing.Span
func (thisFnSpan FnSpan) SetTag(key string, value interface{}) opentracing.Span {
	return thisFnSpan.wrappedSpan.SetTag(key, value)
}

// FnSpan implements opentracing.Span
func (thisFnSpan FnSpan) LogFields(fields ...log.Field) {
	thisFnSpan.wrappedSpan.LogFields(fields...)
}

// FnSpan implements opentracing.Span
func (thisFnSpan FnSpan) LogKV(alternatingKeyValues ...interface{}) {
	thisFnSpan.wrappedSpan.LogKV(alternatingKeyValues...)
}

// FnSpan implements opentracing.Span
func (thisFnSpan FnSpan) SetBaggageItem(restrictedKey, value string) opentracing.Span {
	return thisFnSpan.wrappedSpan.SetBaggageItem(restrictedKey, value)
}

// FnSpan implements opentracing.Span
func (thisFnSpan FnSpan) BaggageItem(restrictedKey string) string {
	return thisFnSpan.wrappedSpan.BaggageItem(restrictedKey)
}

// FnSpan implements opentracing.Span
func (thisFnSpan FnSpan) Tracer() opentracing.Tracer {
	return thisFnSpan.wrappedSpan.Tracer()
}

// FnSpan implements opentracing.Span
func (thisFnSpan FnSpan) LogEvent(event string) {
	thisFnSpan.wrappedSpan.LogEvent(event)
}

// FnSpan implements opentracing.Span
func (thisFnSpan FnSpan) LogEventWithPayload(event string, payload interface{}) {
	thisFnSpan.wrappedSpan.LogEventWithPayload(event, payload)
}

// FnSpan implements opentracing.Span
func (thisFnSpan FnSpan) Log(data opentracing.LogData) {
	thisFnSpan.wrappedSpan.Log(data)
}
