package ochttp

import (
	"testing"

	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
)

func TestVarsInitialized(t *testing.T) {
	// Test that global initialization was successful
	for i, k := range []tag.Key{Host, StatusCode, Path, Method} {
		if k.Name() == "" {
			t.Errorf("key not initialized: %d", i)
		}
	}
	for i, m := range []stats.Measure{ClientRequestCount, ClientResponseBytes, ClientRequestBytes, ClientLatency} {
		if m == nil {
			t.Errorf("measure not initialized: %d", i)
		}
	}
	for i, v := range DefaultViews {
		if v == nil {
			t.Errorf("view not initialized: %d", i)
		}
	}
}
