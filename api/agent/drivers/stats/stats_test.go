package stats

import (
	"testing"
	"time"

	"github.com/fnproject/fn/api/common"
	"github.com/stretchr/testify/assert"
)

func TestAverage(t *testing.T) {
	start := time.Date(2016, 8, 11, 0, 0, 0, 0, time.UTC)
	stats := make([]Stat, 10)
	for i := 0; i < len(stats); i++ {
		stats[i] = Stat{
			Timestamp: common.DateTime(start.Add(time.Duration(i) * time.Minute)),
			Metrics:   map[string]uint64{"x": uint64(i)},
		}
	}

	res, ok := average(stats)
	if !ok {
		t.Error("Expected good record")
	}

	expectedV := uint64(4)
	if v, ok := res.Metrics["x"]; !ok || v != expectedV {
		t.Error("Actual average didn't match expected", "actual", v, "expected", expectedV)
	}

	expectedT := time.Unix(1470873870, 0)
	if time.Time(res.Timestamp) != expectedT {
		t.Error("Actual average didn't match expected", "actual", res.Timestamp, "expected", expectedT)
	}
}

func TestDecimate(t *testing.T) {
	start := time.Now()
	stats := make([]Stat, 480)
	for i := range stats {
		stats[i] = Stat{
			Timestamp: common.DateTime(start.Add(time.Duration(i) * time.Second)),
			Metrics:   map[string]uint64{"x": uint64(i)},
		}
	}

	stats = Decimate(240, stats)
	if len(stats) != 240 {
		t.Error("decimate function bad", len(stats))
	}

	//for i := range stats {
	//t.Log(stats[i])
	//}

	stats = make([]Stat, 700)
	for i := range stats {
		stats[i] = Stat{
			Timestamp: common.DateTime(start.Add(time.Duration(i) * time.Second)),
			Metrics:   map[string]uint64{"x": uint64(i)},
		}
	}
	stats = Decimate(240, stats)
	if len(stats) != 240 {
		t.Error("decimate function bad", len(stats))
	}

	stats = make([]Stat, 300)
	for i := range stats {
		stats[i] = Stat{
			Timestamp: common.DateTime(start.Add(time.Duration(i) * time.Second)),
			Metrics:   map[string]uint64{"x": uint64(i)},
		}
	}
	stats = Decimate(240, stats)
	if len(stats) != 240 {
		t.Error("decimate function bad", len(stats))
	}

	stats = make([]Stat, 300)
	for i := range stats {
		if i == 150 {
			// leave 1 large gap
			start = start.Add(20 * time.Minute)
		}
		stats[i] = Stat{
			Timestamp: common.DateTime(start.Add(time.Duration(i) * time.Second)),
			Metrics:   map[string]uint64{"x": uint64(i)},
		}
	}
	stats = Decimate(240, stats)
	if len(stats) != 49 {
		t.Error("decimate function bad", len(stats))
	}
}

func TestGenerateLogScaleHistogramBucketsWithRange(t *testing.T) {
	assert.Equal(t, []float64{7, 14, 28, 56, 112}, GenerateLogScaleHistogramBucketsWithRange(7, 100))
}

func TestGenerateLinearHistogramBuckets(t *testing.T) {
	assert.Equal(t, []float64{5, 7, 9, 11, 13, 15}, GenerateLinearHistogramBuckets(5, 15, 5))
}

func TestGenerateLogScaleHistogramBuckets(t *testing.T) {
	assert.Equal(t, []float64{0, 0.1953125, 0.390625, 0.78125, 1.5625, 3.125, 6.25, 12.5, 25, 50, 100}, GenerateLogScaleHistogramBuckets(100, 10))
}
