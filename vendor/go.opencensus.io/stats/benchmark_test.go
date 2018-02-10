package stats_test

import (
	"context"
	"log"
	"testing"

	"go.opencensus.io/stats"
	_ "go.opencensus.io/stats/view"
)

var m = makeMeasure()

func BenchmarkRecord(b *testing.B) {
	var ctx = context.Background()
	for i := 0; i < b.N; i++ {
		stats.Record(ctx, m.M(1), m.M(1), m.M(1), m.M(1), m.M(1), m.M(1), m.M(1), m.M(1), m.M(1), m.M(1))
	}
}

func makeMeasure() *stats.Int64Measure {
	m, err := stats.Int64("m", "test measure", "")
	if err != nil {
		log.Fatal(err)
	}
	return m
}
