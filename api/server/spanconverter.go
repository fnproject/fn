package server

import (
	"context"
	"strings"
	"sync"
	"time"
	"unicode"

	"go.opencensus.io/stats"
	view "go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
)

// SpanConverter registers as a opencensus Trace Exporter,
// but it converts all the Spans in Views and registers them as such
// A View exporter will then export them as normal.
type SpanConverter struct {
	opts     Options
	measures map[string]*stats.Float64Measure
	viewsMu  sync.Mutex
	e        view.Exporter
}

// Options contains options for configuring the exporter.
type Options struct {
	Namespace string
}

func NewSpanConverter(o Options) (*SpanConverter, error) {
	c := &SpanConverter{
		opts:     o,
		measures: make(map[string]*stats.Float64Measure),
	}
	return c, nil
}

var maxViews = 100

// Spans are rejected if there are already maxViews (100) or they are
// prefixed with '/', gin as been observed creating Span id specific
// named Spans.
func (c *SpanConverter) rejectSpan(sd *trace.SpanData) bool {
	return len(c.measures) > maxViews || urlName(sd)
}

// ExportSpan creates a Measure and View once per Span.Name, registering
// the View with the opencensus register. The length of time reported
// by the span is then recorded using the measure.
func (c *SpanConverter) ExportSpan(sd *trace.SpanData) {
	if c.rejectSpan(sd) {
		return
	}
	m := c.getMeasure(sd)

	spanTimeNanos := sd.EndTime.Sub(sd.StartTime)
	spanTimeMillis := float64(int64(spanTimeNanos / time.Millisecond))

	stats.Record(context.Background(), m.M(spanTimeMillis))
}

var latencyDist = []float64{1, 10, 50, 100, 250, 500, 1000, 10000, 60000, 120000}

func (c *SpanConverter) getMeasure(span *trace.SpanData) *stats.Float64Measure {
	sig := sanitize(span.Name)
	c.viewsMu.Lock()
	m, ok := c.measures[sig]
	c.viewsMu.Unlock()

	if !ok {
		m = stats.Float64(sig+"_span_time", "The span length in milliseconds", "ms")
		v := &view.View{
			Name:        sanitize(span.Name),
			Description: sanitize(span.Name),
			Measure:     m,
			Aggregation: view.Distribution(latencyDist...),
		}

		c.viewsMu.Lock()
		c.measures[sig] = m
		view.Register(v)
		c.viewsMu.Unlock()
	}

	return m
}

const labelKeySizeLimit = 100

// sanitize returns a string that is trunacated to 100 characters if it's too
// long, and replaces non-alphanumeric characters to underscores.
func sanitize(s string) string {
	if len(s) == 0 {
		return s
	}
	if len(s) > labelKeySizeLimit {
		s = s[:labelKeySizeLimit]
	}
	s = strings.Map(sanitizeRune, s)
	if unicode.IsDigit(rune(s[0])) {
		s = "key_" + s
	}
	if s[0] == '_' {
		s = "key" + s
	}
	return s
}

// converts anything that is not a letter or digit to an underscore
func sanitizeRune(r rune) rune {
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return r
	}
	// Everything else turns into an underscore
	return '_'
}

// Gin creates spans for all paths, containing ID values.
// We can safely discard these, as other histograms are being created for them.
func urlName(sd *trace.SpanData) bool {
	return strings.HasPrefix(sd.Name, "/")
}
