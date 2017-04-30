// Copyright 2016 Iron.io
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package stats

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
)

type HTTPSubHandler interface {
	HTTPHandler(relativeUrl []string, w http.ResponseWriter, r *http.Request)
}

type Config struct {
	Interval float64 `json:"interval" envconfig:"STATS_INTERVAL"` // seconds
	History  int     // minutes

	Log      string `json:"log" envconfig:"STATS_LOG"`
	StatHat  *StatHatReporterConfig
	NewRelic *NewRelicReporterConfig
	Statsd   *StatsdConfig
	GCStats  int `json:"gc_stats" envconfig:"GC_STATS"` // seconds
}

type Statter interface {
	Inc(component string, stat string, value int64, rate float32)
	Gauge(component string, stat string, value int64, rate float32)
	Measure(component string, stat string, value int64, rate float32)
	Time(component string, stat string, value time.Duration, rate float32)
	NewTimer(component string, stat string, rate float32) *Timer
}

type MultiStatter struct {
	statters []Statter
}

func (s *MultiStatter) Inc(component string, stat string, value int64, rate float32) {
	for _, st := range s.statters {
		st.Inc(component, stat, value, rate)
	}
}

func (s *MultiStatter) Gauge(component string, stat string, value int64, rate float32) {
	for _, st := range s.statters {
		st.Gauge(component, stat, value, rate)
	}
}

func (s *MultiStatter) Measure(component string, stat string, value int64, rate float32) {
	for _, st := range s.statters {
		st.Measure(component, stat, value, rate)
	}
}

func (s *MultiStatter) Time(component string, stat string, value time.Duration, rate float32) {
	for _, st := range s.statters {
		st.Time(component, stat, value, rate)
	}
}

func (s *MultiStatter) NewTimer(component string, stat string, rate float32) *Timer {
	return newTimer(s, component, stat, rate)
}

var badDecode error = errors.New("bad stats decode")

func New(config Config) Statter {
	s := new(MultiStatter)

	if config.Interval == 0.0 {
		config.Interval = 10.0 // convenience
	}

	var reporters []reporter
	if config.StatHat != nil && config.StatHat.Email != "" {
		reporters = append(reporters, config.StatHat)
	}

	if config.NewRelic != nil && config.NewRelic.LicenseKey != "" {
		// NR wants version?
		// can get it out of the namespace? roll it here?
		reporters = append(reporters, NewNewRelicReporter("1.0", config.NewRelic.LicenseKey))
	}

	if config.Log != "" {
		reporters = append(reporters, NewLogReporter())
	}

	if len(reporters) > 0 {
		ag := newAggregator(reporters)
		s.statters = append(s.statters, ag)
		go func() {
			for range time.Tick(time.Duration(config.Interval * float64(time.Second))) {
				ag.report(nil)
			}
		}()
	}

	if config.Statsd != nil && config.Statsd.StatsdUdpTarget != "" {
		std, err := NewStatsd(config.Statsd)
		if err == nil {
			s.statters = append(s.statters, std)
		} else {
			logrus.WithError(err).Error("Couldn't create statsd reporter")
		}
	}

	if len(reporters) == 0 && config.Statsd == nil && config.History == 0 {
		return &NilStatter{}
	}

	if config.GCStats >= 0 {
		if config.GCStats == 0 {
			config.GCStats = 1
		}
		go StartReportingMemoryAndGC(s, time.Duration(config.GCStats)*time.Second)
	}

	return s
}

func HTTPReturnJson(w http.ResponseWriter, result interface{}) {
	w.Header().Set("Content-Type", "application/json")
	res, err := json.Marshal(result)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		w.Write(res)
	}
}

// Convert a string to a stat name by replacing '.' with '_', lowercasing the
// string and trimming it. Doesn't do any validation, so do try this out
// locally before sending stats.
func AsStatField(input string) string {
	return strings.Replace(strings.ToLower(strings.TrimSpace(input)), ".", "_", -1)
}

// statsd like API on top of the map manipulation API.
type Timer struct {
	statter   Statter
	component string
	stat      string
	start     time.Time
	rate      float32
	measured  bool
}

func newTimer(st Statter, component, stat string, rate float32) *Timer {
	return &Timer{st, component, stat, time.Now(), rate, false}
}

func (timer *Timer) Measure() {
	if timer.measured {
		return
	}

	timer.measured = true
	timer.statter.Time(timer.component, timer.stat, time.Since(timer.start), timer.rate)
}

type NilStatter struct{}

func (n *NilStatter) Inc(component string, stat string, value int64, rate float32)          {}
func (n *NilStatter) Gauge(component string, stat string, value int64, rate float32)        {}
func (n *NilStatter) Measure(component string, stat string, value int64, rate float32)      {}
func (n *NilStatter) Time(component string, stat string, value time.Duration, rate float32) {}
func (r *NilStatter) NewTimer(component string, stat string, rate float32) *Timer {
	return newTimer(r, component, stat, rate)
}
