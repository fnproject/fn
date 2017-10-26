package server

import (
	"github.com/openzipkin/zipkin-go-opentracing"
	"github.com/openzipkin/zipkin-go-opentracing/thrift/gen-go/zipkincore"
	"github.com/prometheus/client_golang/prometheus"
	"strings"
	"time"
)

// PrometheusCollector is a custom Collector
// which sends ZipKin traces to Prometheus
type PrometheusCollector struct {

	// Each span name is published as a separate Histogram metric
	// Using metric names of the form fn_span_<span-name>_duration_seconds

	// In this map, the key is the name of a tracing span,
	// and the corresponding value is a HistogramVec metric used to report the duration of spans with this name to Prometheus
	histogramVecMap map[string]*prometheus.HistogramVec

	// In this map, the key is the name of a tracing span,
	// and the corresponding value is an array containing the label keys that were specified when the HistogramVec metric was created
	registeredLabelKeysMap map[string][]string
}

// NewPrometheusCollector returns a new PrometheusCollector
func NewPrometheusCollector() (zipkintracer.Collector, error) {
	pc := &PrometheusCollector{make(map[string]*prometheus.HistogramVec), make(map[string][]string)}
	return pc, nil
}

// PrometheusCollector implements Collector.
func (pc PrometheusCollector) Collect(span *zipkincore.Span) error {
	var labelValuesToUse map[string]string

	// extract any label values from the span
	labelKeysFromSpan, labelValuesFromSpan := getLabels(span)

	// get the HistogramVec for this span name
	histogramVec, found := pc.histogramVecMap[span.GetName()]
	if !found {
		// create a new HistogramVec
		histogramVec = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "fn_span_" + span.GetName() + "_duration_seconds",
				Help: "Span " + span.GetName() + " duration, by span name",
			},
			labelKeysFromSpan,
		)
		pc.histogramVecMap[span.GetName()] = histogramVec
		pc.registeredLabelKeysMap[span.GetName()] = labelKeysFromSpan
		prometheus.MustRegister(histogramVec)
		labelValuesToUse = labelValuesFromSpan
	} else {
		// found an existing HistogramVec
		// need to be careful here, since we must supply the same label keys as when we first created the metric
		// otherwise we will get a "inconsistent label cardinality" panic
		// that's why we saved the original label keys in the registeredLabelKeysMap map
		// so we can use that to construct a map of label key/value pairs to set on the metric
		labelValuesToUse = make(map[string]string)
		for _, thisRegisteredLabelKey := range pc.registeredLabelKeysMap[span.GetName()] {
			if value, found := labelValuesFromSpan[thisRegisteredLabelKey]; found {
				labelValuesToUse[thisRegisteredLabelKey] = value
			} else {
				labelValuesToUse[thisRegisteredLabelKey] = ""
			}
		}
	}

	// now report the metric value
	histogramVec.With(labelValuesToUse).Observe((time.Duration(span.GetDuration()) * time.Microsecond).Seconds())

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
