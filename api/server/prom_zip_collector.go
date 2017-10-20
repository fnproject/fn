package server

import (
	"github.com/fnproject/fn/api/agent"
	"github.com/openzipkin/zipkin-go-opentracing"
	"github.com/openzipkin/zipkin-go-opentracing/thrift/gen-go/zipkincore"
	"github.com/prometheus/client_golang/prometheus"
	"strings"
)

// Each span name is published as a separate Histogram metric
// Using metric names of the form fn_span_<span-name>_duration_seconds
var histogramVecMap = make(map[string]*prometheus.HistogramVec)

// Return the HistogramVec corresponding to the specified spanName.
// If a HistogramVec does not already exist for specified spanName then one is created and configured with the specified labels
// otherwise the labels parameter is ignored.
func getHistogramVecForSpanName(spanName string, labels []string) *prometheus.HistogramVec {
	thisHistogramVec, found := histogramVecMap[spanName]
	if !found {
		thisHistogramVec = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "fn_span_" + spanName + "_duration_seconds",
				Help: "Span " + spanName + " duration, by span name",
			},
			labels,
		)
		histogramVecMap[spanName] = thisHistogramVec
		prometheus.MustRegister(thisHistogramVec)
	}
	return thisHistogramVec
}

// PrometheusCollector is a custom Collector
// which sends ZipKin traces to Prometheus
type PrometheusCollector struct {
	a agent.Agent
}

// NewPrometheusCollector returns a new PrometheusCollector
func NewPrometheusCollector(agent agent.Agent) (zipkintracer.Collector, error) {
	pc := &PrometheusCollector{}
	pc.a = agent
	return pc, nil
}

// PrometheusCollector implements Collector.
func (pc PrometheusCollector) Collect(span *zipkincore.Span) error {

	// extract any label values from the span
	labelKeys, labelValueMap := getLabels(span)

	getHistogramVecForSpanName(span.GetName(), labelKeys).With(labelValueMap).Observe(float64(span.GetDuration()) / 1000000)

	return nil
}

// extract from the specified span the key/value pairs that we want to add as labels to the Prometheus metric for this span
// returns an array of keys, and a map of key-value pairs
func getLabels(span *zipkincore.Span) ([]string, map[string]string) {

	var keys []string
	labelMap := make(map[string]string)

	// extract any tags whose key starts with "fn" from the span
	binaryAnnotations := span.GetBinaryAnnotations()
	for _, thisBinaryAnnotation := range binaryAnnotations {
		key := thisBinaryAnnotation.GetKey()
		if thisBinaryAnnotation.GetAnnotationType() == zipkincore.AnnotationType_STRING && strings.HasPrefix(key, "fn") {
			keys = append(keys, key)
			value := string(thisBinaryAnnotation.GetValue()[:])
			labelMap[key] = value
		}
	}

	return keys, labelMap
}

// PrometheusCollector implements Collector.
func (PrometheusCollector) Close() error { return nil }
