package server

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fnproject/fn/api/common"
	"github.com/openzipkin/zipkin-go-opentracing"
	"github.com/openzipkin/zipkin-go-opentracing/thrift/gen-go/zipkincore"
	"github.com/prometheus/client_golang/prometheus"
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
	// and the corresponding value is a CounterVec metric used to report the duration of spans with this name to Prometheus
	counterVecMap map[string]*prometheus.CounterVec

	// In this map, the key is the name of a tracing span,
	// and the corresponding value is a GaugeVec metric used to report the duration of spans with this name to Prometheus
	gaugeVecMap map[string]*prometheus.GaugeVec

	// In this map, the key is the name of a tracing span,
	// and the corresponding value is an array containing the label keys that were specified when the HistogramVec metric was created
	registeredLabelKeysMap map[string][]string
}

// NewPrometheusCollector returns a new PrometheusCollector
func NewPrometheusCollector() (zipkintracer.Collector, error) {
	pc := &PrometheusCollector{
		histogramVecMap:        make(map[string]*prometheus.HistogramVec),
		counterVecMap:          make(map[string]*prometheus.CounterVec),
		gaugeVecMap:            make(map[string]*prometheus.GaugeVec),
		registeredLabelKeysMap: make(map[string][]string),
	}
	return pc, nil
}

// PrometheusCollector implements Collector.
func (pc *PrometheusCollector) Collect(span *zipkincore.Span) error {

	spanName := span.GetName()

	// extract any label values from the span
	labelKeysFromSpan, labelValuesFromSpan := getLabels(span)

	// report the duration of this span as a histogram
	// (unless the span name ends with SpannameSuffixDummy to denote it as being purely the carrier of a metric value and so of no interest in itself)
	if !strings.HasSuffix(spanName, common.SpannameSuffixDummy) {

		// get the HistogramVec for this span name
		histogramVec, labelValuesToUse := pc.getHistogramVec(
			("fn_span_" + spanName + "_duration_seconds"), ("Span " + spanName + " duration, by span name"), labelKeysFromSpan, labelValuesFromSpan)

		// now report the span duration value
		histogramVec.With(labelValuesToUse).Observe((time.Duration(span.GetDuration()) * time.Microsecond).Seconds())

	}

	// now extract any logged histogram metric values from the span
	for key, value := range getLoggedHistogramMetrics(span) {

		// get the HistogramVec for this metric
		thisMetricHistogramVec, labelValuesToUse := pc.getHistogramVec(
			key, ("Metric " + key), labelKeysFromSpan, labelValuesFromSpan)

		// now report the metric value
		thisMetricHistogramVec.With(labelValuesToUse).Observe(value)
	}

	// now extract any logged counter metric values from the span
	for key, value := range getLoggedCounterMetrics(span) {

		// get the CounterVec for this metric
		thisMetricCounterVec, labelValuesToUse := pc.getCounterVec(
			key, ("Metric " + key), labelKeysFromSpan, labelValuesFromSpan)

		// now report the metric value
		thisMetricCounterVec.With(labelValuesToUse).Add(value)
	}

	// now extract any logged gauge metric values from the span
	for key, value := range getLoggedGaugeMetrics(span) {

		// get the GaugeVec for this metric
		thisMetricGaugeVec, labelValuesToUse := pc.getGaugeVec(
			key, ("Metric " + key), labelKeysFromSpan, labelValuesFromSpan)

		// now report the metric value
		thisMetricGaugeVec.With(labelValuesToUse).Add(value)

	}

	return nil
}

// Return (and create, if necessary) a HistogramVec for the specified Prometheus metric
func (pc *PrometheusCollector) getHistogramVec(
	metricName string, metricHelp string, labelKeysFromSpan []string, labelValuesFromSpan map[string]string) (
	*prometheus.HistogramVec, map[string]string) {

	var labelValuesToUse map[string]string

	pc.lock.Lock()
	defer pc.lock.Unlock()

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

// Return (and create, if necessary) a CounterVec for the specified Prometheus metric
func (pc *PrometheusCollector) getCounterVec(
	metricName string, metricHelp string, labelKeysFromSpan []string, labelValuesFromSpan map[string]string) (
	*prometheus.CounterVec, map[string]string) {

	var labelValuesToUse map[string]string

	pc.lock.Lock()
	defer pc.lock.Unlock()

	counterVec, found := pc.counterVecMap[metricName]
	if !found {
		// create a new CounterVec
		counterVec = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: metricName,
				Help: metricHelp,
			},
			labelKeysFromSpan,
		)
		pc.counterVecMap[metricName] = counterVec
		pc.registeredLabelKeysMap[metricName] = labelKeysFromSpan
		prometheus.MustRegister(counterVec)
		labelValuesToUse = labelValuesFromSpan
	} else {
		// found an existing CounterVec
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
	return counterVec, labelValuesToUse
}

// Return (and create, if necessary) a GaugeVec for the specified Prometheus metric
func (pc *PrometheusCollector) getGaugeVec(
	metricName string, metricHelp string, labelKeysFromSpan []string, labelValuesFromSpan map[string]string) (
	*prometheus.GaugeVec, map[string]string) {

	var labelValuesToUse map[string]string

	pc.lock.Lock()
	defer pc.lock.Unlock()

	gaugeVec, found := pc.gaugeVecMap[metricName]
	if !found {
		// create a new GaugeVec
		gaugeVec = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: metricName,
				Help: metricHelp,
			},
			labelKeysFromSpan,
		)
		pc.gaugeVecMap[metricName] = gaugeVec
		pc.registeredLabelKeysMap[metricName] = labelKeysFromSpan
		prometheus.MustRegister(gaugeVec)
		labelValuesToUse = labelValuesFromSpan
	} else {
		// found an existing GaugeVec
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
	return gaugeVec, labelValuesToUse
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

// extract from the span the logged histogram metric values.
// These are the ones whose names start with FieldnamePrefixHistogram,
// and whose values we assume are float64
func getLoggedHistogramMetrics(span *zipkincore.Span) map[string]float64 {

	keyValueMap := make(map[string]float64)

	// extract any annotations whose Value starts with FieldnamePrefixHistogram
	annotations := span.GetAnnotations()
	for _, thisAnnotation := range annotations {
		if strings.HasPrefix(thisAnnotation.GetValue(), common.FieldnamePrefixHistogram) {
			keyvalue := strings.Split(thisAnnotation.GetValue(), "=")
			if len(keyvalue) == 2 {
				if value, err := strconv.ParseFloat(keyvalue[1], 64); err == nil {
					key := strings.TrimSpace(keyvalue[0])
					key = common.FnPrefix + key[len(common.FieldnamePrefixHistogram):] // strip off fieldname prefix and then prepend "fn_" to the front
					keyValueMap[key] = value
				}
			}
		}
	}
	return keyValueMap
}

// extract from the span the logged counter metric values.
// These are the ones whose names start with FieldnamePrefixCounter,
// and whose values we assume are float64
func getLoggedCounterMetrics(span *zipkincore.Span) map[string]float64 {

	keyValueMap := make(map[string]float64)

	// extract any annotations whose Value starts with FieldnamePrefixCounter
	annotations := span.GetAnnotations()
	for _, thisAnnotation := range annotations {
		if strings.HasPrefix(thisAnnotation.GetValue(), common.FieldnamePrefixCounter) {
			keyvalue := strings.Split(thisAnnotation.GetValue(), "=")
			if len(keyvalue) == 2 {
				if value, err := strconv.ParseFloat(keyvalue[1], 64); err == nil {
					key := strings.TrimSpace(keyvalue[0])
					key = common.FnPrefix + key[len(common.FieldnamePrefixCounter):] // strip off fieldname prefix and then prepend "fn_" to the front
					keyValueMap[key] = value
				}
			}
		}
	}
	return keyValueMap
}

// extract from the span the logged gauge metric values.
// These are the ones whose names start with FieldnamePrefixGauge,
// and whose values we assume are float64
func getLoggedGaugeMetrics(span *zipkincore.Span) map[string]float64 {

	keyValueMap := make(map[string]float64)

	// extract any annotations whose Value starts with FieldnamePrefixGauge
	annotations := span.GetAnnotations()
	for _, thisAnnotation := range annotations {
		if strings.HasPrefix(thisAnnotation.GetValue(), common.FieldnamePrefixGauge) {
			keyvalue := strings.Split(thisAnnotation.GetValue(), "=")
			if len(keyvalue) == 2 {
				if value, err := strconv.ParseFloat(keyvalue[1], 64); err == nil {
					key := strings.TrimSpace(keyvalue[0])
					key = common.FnPrefix + key[len(common.FieldnamePrefixGauge):] // strip off fieldname prefix and then prepend "fn_" to the front
					keyValueMap[key] = value
				}
			}
		}
	}
	return keyValueMap
}

// PrometheusCollector implements Collector.
func (*PrometheusCollector) Close() error { return nil }
