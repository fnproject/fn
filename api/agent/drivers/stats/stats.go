// Interface for all container drivers

package stats

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fnproject/fn/api/common"
)

// Stat is a bucket of stats from a driver at a point in time for a certain task.
type Stat struct {
	Timestamp common.DateTime   `json:"timestamp"`
	Metrics   map[string]uint64 `json:"metrics"`
}

// Stats is a list of Stat, notably implements sql.Valuer
type Stats []Stat

// implements sql.Valuer, returning a string
func (s Stats) Value() (driver.Value, error) {
	if len(s) < 1 {
		return driver.Value(string("")), nil
	}
	var b bytes.Buffer
	err := json.NewEncoder(&b).Encode(s)
	// return a string type
	return driver.Value(b.String()), err
}

// implements sql.Scanner
func (s *Stats) Scan(value interface{}) error {
	if value == nil {
		*s = nil
		return nil
	}
	bv, err := driver.String.ConvertValue(value)
	if err == nil {
		var b []byte
		switch x := bv.(type) {
		case []byte:
			b = x
		case string:
			b = []byte(x)
		}

		if len(b) > 0 {
			return json.Unmarshal(b, s)
		}

		*s = nil
		return nil
	}

	// otherwise, return an error
	return fmt.Errorf("stats invalid db format: %T %T value, err: %v", value, bv, err)
}

func average(samples []Stat) (Stat, bool) {
	l := len(samples)
	if l == 0 {
		return Stat{}, false
	} else if l == 1 {
		return samples[0], true
	}

	s := Stat{
		Metrics: samples[0].Metrics, // Recycle Metrics map from first sample
	}
	t := time.Time(samples[0].Timestamp).UnixNano() / int64(l)
	for _, sample := range samples[1:] {
		t += time.Time(sample.Timestamp).UnixNano() / int64(l)
		for k, v := range sample.Metrics {
			s.Metrics[k] += v
		}
	}

	s.Timestamp = common.DateTime(time.Unix(0, t))
	for k, v := range s.Metrics {
		s.Metrics[k] = v / uint64(l)
	}
	return s, true
}

// Decimate will down sample to a max number of points in a given sample by
// averaging samples together. i.e. max=240, if we have 240 samples, return
// them all, if we have 480 samples, every 2 samples average them (and time
// distance), and return 240 samples. This is relatively naive and if len(in) >
// max, <= max points will be returned, not necessarily max: length(out) =
// ceil(length(in)/max) -- feel free to fix this, setting a relatively high max
// will allow good enough granularity at higher lengths, i.e. for max of 1 hour
// tasks, sampling every 1s, decimate will return 15s samples if max=240.
// Large gaps in time between samples (a factor > (last-start)/max) will result
// in a shorter list being returned to account for lost samples.
// Decimate will modify the input list for efficiency, it is not copy safe.
// Input must be sorted by timestamp or this will fail gloriously.
func Decimate(maxSamples int, stats []Stat) []Stat {
	if len(stats) <= maxSamples {
		return stats
	} else if maxSamples <= 0 { // protect from nefarious input
		return nil
	}

	start := time.Time(stats[0].Timestamp)
	window := time.Time(stats[len(stats)-1].Timestamp).Sub(start) / time.Duration(maxSamples)

	nextEntry, current := 0, start // nextEntry is the index tracking next Stats record location
	for x := 0; x < len(stats); {
		isLastEntry := nextEntry == maxSamples-1 // Last bin is larger than others to handle imprecision

		var samples []Stat
		for offset := 0; x+offset < len(stats); offset++ { // Iterate through samples until out of window
			if !isLastEntry && time.Time(stats[x+offset].Timestamp).After(current.Add(window)) {
				break
			}
			samples = stats[x : x+offset+1]
		}

		x += len(samples)                      // Skip # of samples for next window
		if entry, ok := average(samples); ok { // Only record Stat if 1+ samples exist
			stats[nextEntry] = entry
			nextEntry++
		}

		current = current.Add(window)
	}
	return stats[:nextEntry] // Return slice of []Stats that was modified with averages
}
