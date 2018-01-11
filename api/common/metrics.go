package common

import (
	"context"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
)

// IncrementGauge increments the specified gauge metric
// It does this by logging an appropriate field value to a tracing span.
func IncrementGauge(ctx context.Context, metric string) {
	// The field name we use is the specified metric name prepended with "fn_gauge_" to designate that it is a Prometheus gauge metric
	// The collector will remove "gauge_" and use the result as the Prometheus metric name.
	fieldname := "fn_gauge_" + metric

	// Spans are not processed by the collector until the span ends, so to prevent any delay
	// in processing the stats when the current span is long-lived we create a new span for every call
	// suffix the span name with "_dummy" to denote that it is used only to hold a metric and isn't itself of any interest
	span, ctx := opentracing.StartSpanFromContext(ctx, fieldname+"_dummy")
	defer span.Finish()

	// gauge metrics are actually float64; here we log that it should be increased by +1
	span.LogFields(log.Float64(fieldname, 1.))
}

// DecrementGauge decrements the specified gauge metric
// It does this by logging an appropriate field value to a tracing span.
func DecrementGauge(ctx context.Context, metric string) {
	// The field name we use is the specified metric name prepended with "fn_gauge_" to designate that it is a Prometheus gauge metric
	// The collector will remove "gauge_" and use the result as the Prometheus metric name.
	fieldname := "fn_gauge_" + metric

	// Spans are not processed by the collector until the span ends, so to prevent any delay
	// in processing the stats when the current span is long-lived we create a new span for every call.
	// Suffix the span name with "_dummy" to denote that it is used only to hold a metric and isn't itself of any interest
	span, ctx := opentracing.StartSpanFromContext(ctx, fieldname+"_dummy")
	defer span.Finish()

	// gauge metrics are actually float64; here we log that it should be increased by -1
	span.LogFields(log.Float64(fieldname, -1.))
}

// IncrementCounter increments the specified counter metric
// It does this by logging an appropriate field value to a tracing span.
func IncrementCounter(ctx context.Context, metric string) {
	// The field name we use is the specified metric name prepended with "fn_counter_" to designate that it is a Prometheus counter metric
	// The collector will remove "fn_counter_" and use the result as the Prometheus metric name.
	fieldname := "fn_counter_" + metric

	// Spans are not processed by the collector until the span ends, so to prevent any delay
	// in processing the stats when the current span is long-lived we create a new span for every call.
	// Suffix the span name with "_dummy" to denote that it is used only to hold a metric and isn't itself of any interest
	span, ctx := opentracing.StartSpanFromContext(ctx, fieldname+"_dummy")
	defer span.Finish()

	// counter metrics are actually float64; here we log that it should be increased by +1
	span.LogFields(log.Float64(fieldname, 1.))
}

// PublishHistograms publishes the specifed histogram metrics
// It does this by logging appropriate field values to a tracing span
func PublishHistograms(ctx context.Context, metrics map[string]float64) {

	// Spans are not processed by the collector until the span ends, so to prevent any delay
	// in processing the stats when the current span is long-lived we create a new span for every call.
	// Suffix the span name with "_dummy" to denote that it is used only to hold metrics and isn't itself of any interest
	span, ctx := opentracing.StartSpanFromContext(ctx, "histogram_metrics_dummy")
	defer span.Finish()

	for key, value := range metrics {
		// The field name we use is the metric name prepended with "fn_histogram_" to designate that it is a Prometheus histogram metric
		// The collector will remove "histogram_" and use the result as the Prometheus metric name.
		fieldname := "fn_histogram_" + key

		span.LogFields(log.Float64(fieldname, value))
	}
}

// If required, create a scalar version of PublishHistograms that publishes a single histogram metric
