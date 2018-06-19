package drivers

import (
	"testing"
	"time"

	"github.com/fnproject/fn/api/common"
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

func TestParseImage(t *testing.T) {
	cases := map[string][]string{
		"fnproject/fn-test-utils":      {"", "fnproject/fn-test-utils", "latest"},
		"fnproject/fn-test-utils:v1":   {"", "fnproject/fn-test-utils", "v1"},
		"my.registry/fn-test-utils":    {"my.registry", "fn-test-utils", "latest"},
		"my.registry/fn-test-utils:v1": {"my.registry", "fn-test-utils", "v1"},
		"mongo":                                                               {"", "library/mongo", "latest"},
		"mongo:v1":                                                            {"", "library/mongo", "v1"},
		"quay.com/fnproject/fn-test-utils":                                    {"quay.com", "fnproject/fn-test-utils", "latest"},
		"quay.com:8080/fnproject/fn-test-utils:v2":                            {"quay.com:8080", "fnproject/fn-test-utils", "v2"},
		"localhost.localdomain:5000/samalba/hipache:latest":                   {"localhost.localdomain:5000", "samalba/hipache", "latest"},
		"localhost.localdomain:5000/samalba/hipache/isthisallowedeven:latest": {"localhost.localdomain:5000", "samalba/hipache/isthisallowedeven", "latest"},
	}

	for in, out := range cases {
		reg, repo, tag := ParseImage(in)
		if reg != out[0] || repo != out[1] || tag != out[2] {
			t.Errorf("Test input %q wasn't parsed as expected. Expected %q, got %q", in, out, []string{reg, repo, tag})
		}
	}
}
