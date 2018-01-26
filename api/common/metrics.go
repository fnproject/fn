package common

import (
	"context"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
)

// IncrementGauge increments the specified gauge metric
// It does this by logging an appropriate field value to a tracing span.
func IncrementGauge(ctx context.Context, metric string) {
	// The field name we use is the specified metric name prepended with FieldnamePrefixGauge to designate that it is a Prometheus gauge metric
	// The collector will replace that prefix with "fn_" and use the result as the Prometheus metric name.
	fieldname := FieldnamePrefixGauge + metric

	// Spans are not processed by the collector until the span ends, so to prevent any delay
	// in processing the stats when the current span is long-lived we create a new span for every call
	// suffix the span name with SpannameSuffixDummy to denote that it is used only to hold a metric and isn't itself of any interest
	span, ctx := opentracing.StartSpanFromContext(ctx, fieldname+SpannameSuffixDummy)
	defer span.Finish()

	// gauge metrics are actually float64; here we log that it should be increased by +1
	span.LogFields(log.Float64(fieldname, 1.))
}

// DecrementGauge decrements the specified gauge metric
// It does this by logging an appropriate field value to a tracing span.
func DecrementGauge(ctx context.Context, metric string) {
	// The field name we use is the specified metric name prepended with FieldnamePrefixGauge to designate that it is a Prometheus gauge metric
	// The collector will replace that prefix with "fn_" and use the result as the Prometheus metric name.
	fieldname := FieldnamePrefixGauge + metric

	// Spans are not processed by the collector until the span ends, so to prevent any delay
	// in processing the stats when the current span is long-lived we create a new span for every call.
	// suffix the span name with SpannameSuffixDummy to denote that it is used only to hold a metric and isn't itself of any interest
	span, ctx := opentracing.StartSpanFromContext(ctx, fieldname+SpannameSuffixDummy)
	defer span.Finish()

	// gauge metrics are actually float64; here we log that it should be increased by -1
	span.LogFields(log.Float64(fieldname, -1.))
}

// IncrementCounter increments the specified counter metric
// It does this by logging an appropriate field value to a tracing span.
func IncrementCounter(ctx context.Context, metric string) {
	// The field name we use is the specified metric name prepended with FieldnamePrefixCounter to designate that it is a Prometheus counter metric
	// The collector will replace that prefix with "fn_" and use the result as the Prometheus metric name.
	fieldname := FieldnamePrefixCounter + metric

	// Spans are not processed by the collector until the span ends, so to prevent any delay
	// in processing the stats when the current span is long-lived we create a new span for every call.
	// suffix the span name with SpannameSuffixDummy to denote that it is used only to hold a metric and isn't itself of any interest
	span, ctx := opentracing.StartSpanFromContext(ctx, fieldname+SpannameSuffixDummy)
	defer span.Finish()

	// counter metrics are actually float64; here we log that it should be increased by +1
	span.LogFields(log.Float64(fieldname, 1.))
}

// If required, create a scalar version of PublishHistograms that publishes a single histogram metric

// PublishHistograms publishes the specifed histogram metrics
// It does this by logging appropriate field values to a tracing span
// Use this when the current tracing span is long-lived and you want the metric to be visible before it ends
func PublishHistograms(ctx context.Context, metrics map[string]float64) {

	// Spans are not processed by the collector until the span ends, so to prevent any delay
	// in processing the stats when the current span is long-lived we create a new span for every call.
	// suffix the span name with SpannameSuffixDummy to denote that it is used only to hold a metric and isn't itself of any interest
	span, ctx := opentracing.StartSpanFromContext(ctx, "histogram_metrics"+SpannameSuffixDummy)
	defer span.Finish()

	for key, value := range metrics {
		// The field name we use is the metric name prepended with FieldnamePrefixHistogram to designate that it is a Prometheus histogram metric
		// The collector will replace that prefix with "fn_" and use the result as the Prometheus metric name.
		fieldname := FieldnamePrefixHistogram + key
		span.LogFields(log.Float64(fieldname, value))
	}
}

// PublishHistogram publishes the specifed histogram metric
// It does this by logging an appropriate field value to a tracing span
// Use this when the current tracing span is long-lived and you want the metric to be visible before it ends
func PublishHistogram(ctx context.Context, key string, value float64) {

	// Spans are not processed by the collector until the span ends, so to prevent any delay
	// in processing the stats when the current span is long-lived we create a new span for every call.
	// suffix the span name with SpannameSuffixDummy to denote that it is used only to hold a metric and isn't itself of any interest
	span, ctx := opentracing.StartSpanFromContext(ctx, "histogram_metrics"+SpannameSuffixDummy)
	defer span.Finish()

	// The field name we use is the metric name prepended with FieldnamePrefixHistogram to designate that it is a Prometheus histogram metric
	// The collector will replace that prefix with "fn_" and use the result as the Prometheus metric name.
	fieldname := FieldnamePrefixHistogram + key
	span.LogFields(log.Float64(fieldname, value))
}

// PublishHistogramToSpan publishes the specifed histogram metric
// It does this by logging an appropriate field value to the specified tracing span
// Use this when you don't need to create a new tracing span
func PublishHistogramToSpan(span opentracing.Span, key string, value float64) {

	// The field name we use is the metric name prepended with FieldnamePrefixHistogram to designate that it is a Prometheus histogram metric
	// The collector will replace that prefix with "fn_" and use the result as the Prometheus metric name.
	fieldname := FieldnamePrefixHistogram + key
	span.LogFields(log.Float64(fieldname, value))
}

// PublishElapsedTimeToSpan publishes the specifed histogram elapsed time since start
// It does this by logging an appropriate field value to a tracing span
// Use this when the current tracing span is long-lived and you want the metric to be visible before it ends
func PublishElapsedTimeHistogram(ctx context.Context, key string, start, end time.Time) {
	elapsed := float64(end.Sub(start).Seconds())
	PublishHistogram(ctx, key, elapsed)
}

const (

	// FnPrefix is a constant for "fn_", used as a prefix for span names, field names, Prometheus metric names and Prometheus label names
	FnPrefix = "fn_"

	// FieldnamePrefixHistogram is prefixed to the name of a logged field
	// to denote that it corresponds to a histogram metric
	FieldnamePrefixHistogram = FnPrefix + "histogram_"

	// FieldnamePrefixCounter is prefixed to the name of a logged field
	// to denote that it corresponds to a counter metric
	FieldnamePrefixCounter = FnPrefix + "counter_"

	// FieldnamePrefixGauge is prefixed to the name of a logged field
	// to denote that it corresponds to a gauge metric
	FieldnamePrefixGauge = FnPrefix + "gauge_"

	// SpannameSuffixDummy is suffixed to the name of a tracing span
	// to denote that it has been created solely for the purpose of carrying metric values
	// and is not itself of any interest and should not be converted to a Prometheus duration metric
	SpannameSuffixDummy = "_dummy"
)
