// Copyright 2017, OpenCensus Authors
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

package prometheus

import (
	"sync"
	"testing"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"

	"github.com/prometheus/client_golang/prometheus"
)

func newView(agg view.Aggregation) *view.View {
	m, _ := stats.Int64("tests/foo1", "bytes", "byte")
	view, _ := view.New("foo", "bar", nil, m, agg)
	return view
}

func TestOnlyCumulativeWindowSupported(t *testing.T) {
	// See Issue https://github.com/census-instrumentation/opencensus-go/issues/214.
	count1 := view.CountData(1)
	mean1 := view.MeanData{
		Mean:  4.5,
		Count: 5,
	}
	tests := []struct {
		vds  *view.Data
		want int
	}{
		0: {
			vds: &view.Data{
				View: newView(view.CountAggregation{}),
			},
			want: 0, // no rows present
		},
		1: {
			vds: &view.Data{
				View: newView(view.CountAggregation{}),
				Rows: []*view.Row{
					{Data: &count1},
				},
			},
			want: 1,
		},
		2: {
			vds: &view.Data{
				View: newView(view.MeanAggregation{}),
				Rows: []*view.Row{
					{Data: &mean1},
				},
			},
			want: 1,
		},
	}

	for i, tt := range tests {
		reg := prometheus.NewRegistry()
		collector := newCollector(Options{}, reg)
		collector.addViewData(tt.vds)
		mm, err := reg.Gather()
		if err != nil {
			t.Errorf("#%d: Gather err: %v", i, err)
		}
		reg.Unregister(collector)
		if got, want := len(mm), tt.want; got != want {
			t.Errorf("#%d: got nil %v want nil %v", i, got, want)
		}
	}
}

func TestSingletonExporter(t *testing.T) {
	exp, err := NewExporter(Options{})
	if err != nil {
		t.Fatalf("NewExporter() = %v", err)
	}
	if exp == nil {
		t.Fatal("Nil exporter")
	}

	// Should all now fail
	exp, err = NewExporter(Options{})
	if err == nil {
		t.Fatal("NewExporter() = nil")
	}
	if exp != nil {
		t.Fatal("Non-nil exporter")
	}
}

func TestCollectNonRacy(t *testing.T) {
	// Despite enforcing the singleton, for this case we
	// need an exporter hence won't be using NewExporter.
	exp, err := newExporter(Options{})
	if err != nil {
		t.Fatalf("NewExporter: %v", err)
	}
	collector := exp.c

	// Synchronize and make sure every goroutine has terminated before we exit
	var waiter sync.WaitGroup
	waiter.Add(3)
	defer waiter.Wait()

	doneCh := make(chan bool)
	// 1. Viewdata write routine at 700ns
	go func() {
		defer waiter.Done()
		tick := time.NewTicker(700 * time.Nanosecond)
		defer tick.Stop()

		defer func() {
			close(doneCh)
		}()

		for i := 0; i < 1e3; i++ {
			count1 := view.CountData(1)
			mean1 := &view.MeanData{Mean: 4.5, Count: 5}
			vds := []*view.Data{
				{View: newView(view.MeanAggregation{}), Rows: []*view.Row{{Data: mean1}}},
				{View: newView(view.CountAggregation{}), Rows: []*view.Row{{Data: &count1}}},
			}
			for _, v := range vds {
				exp.ExportView(v)
			}
			<-tick.C
		}
	}()

	inMetricsChan := make(chan prometheus.Metric, 1000)
	// 2. Simulating the Prometheus metrics consumption routine running at 900ns
	go func() {
		defer waiter.Done()
		tick := time.NewTicker(900 * time.Nanosecond)
		defer tick.Stop()

		for {
			select {
			case <-doneCh:
				return
			case <-inMetricsChan:
			}
		}
	}()

	// 3. Collect/Read routine at 800ns
	go func() {
		defer waiter.Done()
		tick := time.NewTicker(800 * time.Nanosecond)
		defer tick.Stop()

		for {
			select {
			case <-doneCh:
				return
			case <-tick.C:
				// Perform some collection here
				collector.Collect(inMetricsChan)
			}
		}
	}()
}
