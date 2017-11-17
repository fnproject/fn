package server

import (
	"github.com/openzipkin/zipkin-go-opentracing"
	"github.com/openzipkin/zipkin-go-opentracing/thrift/gen-go/zipkincore"
	"github.com/prometheus/client_golang/prometheus"
	"strconv"
	"strings"
	"sync"
	"time"
)

// PrometheusCollector is a custom Collector
// which sends ZipKin traces to Prometheus
type PrometheusCollector struct {
	lock sync.Mutex
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
	pc := &PrometheusCollector{
		histogramVecMap:        make(map[string]*prometheus.HistogramVec),
		registeredLabelKeysMap: make(map[string][]string),
	}
	return pc, nil
}

// PrometheusCollector implements Collector.
func (pc *PrometheusCollector) Collect(span *zipkincore.Span) error {

	pc.lock.Lock()
	defer pc.lock.Unlock()

	spanName := span.GetName()

	// extract any label values from the span
	labelKeysFromSpan, labelValuesFromSpan := getLabels(span)

	// get the HistogramVec for this span name
	histogramVec, labelValuesToUse := pc.getHistogramVec(
		("fn_span_" + spanName + "_duration_seconds"), ("Span " + spanName + " duration, by span name"), labelKeysFromSpan, labelValuesFromSpan)

	// now report the span duration value
	histogramVec.With(labelValuesToUse).Observe((time.Duration(span.GetDuration()) * time.Microsecond).Seconds())

	// now extract any logged metric values from the span
	for key, value := range getLoggedMetrics(span) {

		// get the HistogramVec for this metric
		thisMetricHistogramVec, labelValuesToUse := pc.getHistogramVec(
			("fn_" + spanName + "_" + key), (spanName + " metric " + key), labelKeysFromSpan, labelValuesFromSpan)

		// now report the metric value
		thisMetricHistogramVec.With(labelValuesToUse).Observe(float64(value))
	}

	return nil
}

// Return (and create, if necessary) a HistogramVec for the specified Prometheus metric
func (pc *PrometheusCollector) getHistogramVec(
	metricName string, metricHelp string, labelKeysFromSpan []string, labelValuesFromSpan map[string]string) (
	*prometheus.HistogramVec, map[string]string) {

	var labelValuesToUse map[string]string

	histogramVec, found := pc.histogramVecMap[metricName]
	if !found {
		// create a new HistogramVec
		histogramVec = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: metricName,
				Help: metricHelp,
			},
			labelKeysFromSpan,
		)
		pc.histogramVecMap[metricName] = histogramVec
		pc.registeredLabelKeysMap[metricName] = labelKeysFromSpan
		prometheus.MustRegister(histogramVec)
		labelValuesToUse = labelValuesFromSpan
	} else {
		// found an existing HistogramVec
		// need to be careful here, since we must supply the same label keys as when we first created the metric
		// otherwise we will get a "inconsistent label cardinality" panic
		// that's why we saved the original label keys in the registeredLabelKeysMap map
		// so we can use that to construct a map of label key/value pairs to set on the metric
		labelValuesToUse = make(map[string]string)
		for _, thisRegisteredLabelKey := range pc.registeredLabelKeysMap[metricName] {
			if value, found := labelValuesFromSpan[thisRegisteredLabelKey]; found {
				labelValuesToUse[thisRegisteredLabelKey] = value
			} else {
				labelValuesToUse[thisRegisteredLabelKey] = ""
			}
		}
	}
	return histogramVec, labelValuesToUse
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

// extract from the span the logged metric values, which we assume as uint64 values
func getLoggedMetrics(span *zipkincore.Span) map[string]uint64 {

	keyValueMap := make(map[string]uint64)

	// extract any annotations whose Value starts with "fn_"
	annotations := span.GetAnnotations()
	for _, thisAnnotation := range annotations {
		if strings.HasPrefix(thisAnnotation.GetValue(), "fn_") {
			keyvalue := strings.Split(thisAnnotation.GetValue(), "=")
			if len(keyvalue) == 2 {
				if value, err := strconv.ParseUint(keyvalue[1], 10, 64); err == nil {
					key := strings.TrimSpace(keyvalue[0])
					key = key[3:] // strip off leading fn_
					keyValueMap[key] = value
				}
			}
		}
	}
	return keyValueMap
}

// PrometheusCollector implements Collector.
func (*PrometheusCollector) Close() error { return nil }
