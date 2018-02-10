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
//

package view

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
)

func Test_Worker_MeasureCreation(t *testing.T) {
	restart()

	if _, err := stats.Float64("MF1", "desc MF1", "unit"); err != nil {
		t.Errorf("stats.Float64(\"MF1\", \"desc MF1\") got error %v, want no error", err)
	}

	if _, err := stats.Float64("MF1", "Duplicate measure with same name as MF1.", "unit"); err == nil {
		t.Error("stats.Float64(\"MF1\", \"Duplicate Float64Measure with same name as MF1.\") got no error, want no error")
	}

	if _, err := stats.Int64("MF1", "Duplicate measure with same name as MF1.", "unit"); err == nil {
		t.Error("stats.Int64(\"MF1\", \"Duplicate Int64Measure with same name as MF1.\") got no error, want no error")
	}

	if _, err := stats.Float64("MF2", "desc MF2", "unit"); err != nil {
		t.Errorf("stats.Float64(\"MF2\", \"desc MF2\") got error %v, want no error", err)
	}

	if _, err := stats.Int64("MI1", "desc MI1", "unit"); err != nil {
		t.Errorf("stats.Int64(\"MI1\", \"desc MI1\") got error %v, want no error", err)
	}

	if _, err := stats.Int64("MI1", "Duplicate measure with same name as MI1.", "unit"); err == nil {
		t.Error("stats.Int64(\"MI1\", \"Duplicate Int64 with same name as MI1.\") got no error, want no error")
	}

	if _, err := stats.Float64("MI1", "Duplicate measure with same name as MI1.", "unit"); err == nil {
		t.Error("stats.Float64(\"MI1\", \"Duplicate Float64 with same name as MI1.\") got no error, want no error")
	}
}

func Test_Worker_ViewRegistration(t *testing.T) {
	someError := errors.New("some error")

	sc1 := make(chan *Data)

	type registerWant struct {
		vID string
		err error
	}
	type unregisterWant struct {
		vID string
		err error
	}
	type byNameWant struct {
		name string
		vID  string
		ok   bool
	}

	type subscription struct {
		c   chan *Data
		vID string
		err error
	}
	type testCase struct {
		label         string
		regs          []registerWant
		subscriptions []subscription
		unregs        []unregisterWant
		bynames       []byNameWant
	}
	tcs := []testCase{
		{
			"register and subscribe to v1ID",
			[]registerWant{
				{
					"v1ID",
					nil,
				},
			},
			[]subscription{
				{
					sc1,
					"v1ID",
					nil,
				},
			},
			[]unregisterWant{
				{
					"v1ID",
					someError,
				},
			},
			[]byNameWant{
				{
					"VF1",
					"v1ID",
					true,
				},
				{
					"VF2",
					"vNilID",
					false,
				},
			},
		},
		{
			"register v1ID+v2ID, susbsribe to v1ID",
			[]registerWant{
				{
					"v1ID",
					nil,
				},
				{
					"v2ID",
					nil,
				},
			},
			[]subscription{
				{
					sc1,
					"v1ID",
					nil,
				},
			},
			[]unregisterWant{
				{
					"v1ID",
					someError,
				},
				{
					"v2ID",
					someError,
				},
			},
			[]byNameWant{
				{
					"VF1",
					"v1ID",
					true,
				},
				{
					"VF2",
					"v2ID",
					true,
				},
			},
		},
		{
			"register to v1ID; subscribe to v1ID and view with same ID",
			[]registerWant{
				{
					"v1ID",
					nil,
				},
			},
			[]subscription{
				{
					sc1,
					"v1ID",
					nil,
				},
				{
					sc1,
					"v1SameNameID",
					someError,
				},
			},
			[]unregisterWant{
				{
					"v1ID",
					someError,
				},
				{
					"v1SameNameID",
					nil,
				},
			},
			[]byNameWant{
				{
					"VF1",
					"v1ID",
					true,
				},
			},
		},
	}

	mf1, _ := stats.Float64("MF1/Test_Worker_ViewRegistration", "desc MF1", "unit")
	mf2, _ := stats.Float64("MF2/Test_Worker_ViewRegistration", "desc MF2", "unit")

	for _, tc := range tcs {
		t.Run(tc.label, func(t *testing.T) {
			restart()

			v1, _ := New("VF1", "desc VF1", nil, mf1, nil)
			v11, _ := New("VF1", "desc duplicate name VF1", nil, mf1, nil)
			v2, _ := New("VF2", "desc VF2", nil, mf2, nil)

			views := map[string]*View{
				"v1ID":         v1,
				"v1SameNameID": v11,
				"v2ID":         v2,
				"vNilID":       nil,
			}

			for _, reg := range tc.regs {
				v := views[reg.vID]
				err := Register(v)
				if (err != nil) != (reg.err != nil) {
					t.Errorf("%v: Register() = %v, want %v", tc.label, err, reg.err)
				}
				v.subscribe()
			}

			for _, s := range tc.subscriptions {
				v := views[s.vID]
				err := v.Subscribe()
				if (err != nil) != (s.err != nil) {
					t.Errorf("%v: Subscribe() = %v, want %v", tc.label, err, s.err)
				}
			}

			for _, unreg := range tc.unregs {
				v := views[unreg.vID]
				err := Unregister(v)
				if (err != nil) != (unreg.err != nil) {
					t.Errorf("%v: Unregister() = %v; want %v", tc.label, err, unreg.err)
				}
			}

			for _, byname := range tc.bynames {
				v := Find(byname.name)
				if v == nil && byname.ok {
					t.Errorf("%v: ViewByName(%q) = nil, want non-nil view", tc.label, byname.name)
				}

				wantV := views[byname.vID]
				if v != wantV {
					t.Errorf("%v: ViewByName(%q) = %v; want %v", tc.label, byname.name, v, wantV)
				}
			}
		})
	}
}

func Test_Worker_RecordFloat64(t *testing.T) {
	restart()

	someError := errors.New("some error")
	m, err := stats.Float64("Test_Worker_RecordFloat64/MF1", "desc MF1", "unit")
	if err != nil {
		t.Errorf("stats.Float64(\"MF1\", \"desc MF1\") got error '%v', want no error", err)
	}

	k1, _ := tag.NewKey("k1")
	k2, _ := tag.NewKey("k2")
	ctx, err := tag.New(context.Background(),
		tag.Insert(k1, "v1"),
		tag.Insert(k2, "v2"),
	)
	if err != nil {
		t.Fatal(err)
	}

	v1, err := New("VF1", "desc VF1", []tag.Key{k1, k2}, m, CountAggregation{})
	if err != nil {
		t.Fatal(err)
	}
	v2, err := New("VF2", "desc VF2", []tag.Key{k1, k2}, m, CountAggregation{})
	if err != nil {
		t.Fatal(err)
	}

	type want struct {
		v    *View
		rows []*Row
		err  error
	}
	type testCase struct {
		label         string
		registrations []*View
		subscriptions []*View
		records       []float64
		wants         []want
	}

	tcs := []testCase{
		{
			"0",
			[]*View{v1, v2},
			[]*View{},
			[]float64{1, 1},
			[]want{{v1, nil, someError}, {v2, nil, someError}},
		},
		{
			"1",
			[]*View{v1, v2},
			[]*View{v1},
			[]float64{1, 1},
			[]want{
				{
					v1,
					[]*Row{
						{
							[]tag.Tag{{Key: k1, Value: "v1"}, {Key: k2, Value: "v2"}},
							newCountData(2),
						},
					},
					nil,
				},
				{v2, nil, someError},
			},
		},
		{
			"2",
			[]*View{v1, v2},
			[]*View{v1, v2},
			[]float64{1, 1},
			[]want{
				{
					v1,
					[]*Row{
						{
							[]tag.Tag{{Key: k1, Value: "v1"}, {Key: k2, Value: "v2"}},
							newCountData(2),
						},
					},
					nil,
				},
				{
					v2,
					[]*Row{
						{
							[]tag.Tag{{Key: k1, Value: "v1"}, {Key: k2, Value: "v2"}},
							newCountData(2),
						},
					},
					nil,
				},
			},
		},
	}

	for _, tc := range tcs {
		for _, v := range tc.registrations {
			if err := Register(v); err != nil {
				t.Fatalf("%v: Register(%v) = %v; want no errors", tc.label, v.Name(), err)
			}
		}

		for _, v := range tc.subscriptions {
			if err := v.Subscribe(); err != nil {
				t.Fatalf("%v: Subscribe(%v) = %v; want no errors", tc.label, v.Name(), err)
			}
		}

		for _, value := range tc.records {
			stats.Record(ctx, m.M(value))
		}

		for _, w := range tc.wants {
			gotRows, err := w.v.RetrieveData()
			if (err != nil) != (w.err != nil) {
				t.Fatalf("%v: RetrieveData(%v) = %v; want no errors", tc.label, w.v.Name(), err)
			}
			for _, got := range gotRows {
				if !containsRow(w.rows, got) {
					t.Errorf("%v: got row %v; want none", tc.label, got)
					break
				}
			}
			for _, want := range w.rows {
				if !containsRow(gotRows, want) {
					t.Errorf("%v: got none; want %v'", tc.label, want)
					break
				}
			}
		}

		// cleaning up
		for _, v := range tc.subscriptions {
			if err := v.Unsubscribe(); err != nil {
				t.Fatalf("%v: Unsubscribing from view %v errored with %v; want no error", tc.label, v.Name(), err)
			}
		}

		for _, v := range tc.registrations {
			if err := Unregister(v); err != nil {
				t.Fatalf("%v: Unregistering view %v errrored with %v; want no error", tc.label, v.Name(), err)
			}
		}
	}
}

func TestReportUsage(t *testing.T) {
	ctx := context.Background()

	m, err := stats.Int64("measure", "desc", "unit")
	if err != nil {
		t.Fatalf("stats.Int64() = %v", err)
	}

	cum1, _ := New("cum1", "", nil, m, CountAggregation{})
	cum2, _ := New("cum1", "", nil, m, CountAggregation{})

	tests := []struct {
		name         string
		view         *View
		wantMaxCount int64
	}{
		{
			name:         "cum",
			view:         cum1,
			wantMaxCount: 8,
		},
		{
			name:         "cum2",
			view:         cum2,
			wantMaxCount: 8,
		},
	}

	for _, tt := range tests {
		restart()
		SetReportingPeriod(25 * time.Millisecond)

		if err := tt.view.Subscribe(); err != nil {
			t.Fatalf("%v: cannot subscribe: %v", tt.name, err)
		}

		e := &countExporter{}
		RegisterExporter(e)

		stats.Record(ctx, m.M(1))
		stats.Record(ctx, m.M(1))
		stats.Record(ctx, m.M(1))
		stats.Record(ctx, m.M(1))

		time.Sleep(50 * time.Millisecond)

		stats.Record(ctx, m.M(1))
		stats.Record(ctx, m.M(1))
		stats.Record(ctx, m.M(1))
		stats.Record(ctx, m.M(1))

		time.Sleep(50 * time.Millisecond)

		e.Lock()
		count := e.count
		e.Unlock()
		if got, want := count, tt.wantMaxCount; got > want {
			t.Errorf("%v: got count data = %v; want at most %v", tt.name, got, want)
		}
	}

}

func Test_SetReportingPeriodReqNeverBlocks(t *testing.T) {
	t.Parallel()

	worker := newWorker()
	durations := []time.Duration{-1, 0, 10, 100 * time.Millisecond}
	for i, duration := range durations {
		ackChan := make(chan bool, 1)
		cmd := &setReportingPeriodReq{c: ackChan, d: duration}
		cmd.handleCommand(worker)

		select {
		case <-ackChan:
		case <-time.After(500 * time.Millisecond): // Arbitrarily using 500ms as the timeout duration.
			t.Errorf("#%d: duration %v blocks", i, duration)
		}
	}
}

func TestWorkerStarttime(t *testing.T) {
	restart()

	ctx := context.Background()
	m, err := stats.Int64("measure/TestWorkerStarttime", "desc", "unit")
	if err != nil {
		t.Fatalf("stats.Int64() = %v", err)
	}
	v, err := New("testview", "", nil, m, CountAggregation{})
	if err != nil {
		t.Fatalf("New() = %v", err)
	}

	SetReportingPeriod(25 * time.Millisecond)
	if err := v.Subscribe(); err != nil {
		t.Fatalf("cannot subscribe to %v: %v", v.Name(), err)
	}

	e := &vdExporter{}
	RegisterExporter(e)
	defer UnregisterExporter(e)

	stats.Record(ctx, m.M(1))
	stats.Record(ctx, m.M(1))
	stats.Record(ctx, m.M(1))
	stats.Record(ctx, m.M(1))

	time.Sleep(50 * time.Millisecond)

	stats.Record(ctx, m.M(1))
	stats.Record(ctx, m.M(1))
	stats.Record(ctx, m.M(1))
	stats.Record(ctx, m.M(1))

	time.Sleep(50 * time.Millisecond)

	e.Lock()
	if len(e.vds) == 0 {
		t.Fatal("Got no view data; want at least one")
	}

	var start time.Time
	for _, vd := range e.vds {
		if start.IsZero() {
			start = vd.Start
		}
		if !vd.Start.Equal(start) {
			t.Errorf("Cumulative view data start time = %v; want %v", vd.Start, start)
		}
	}
	e.Unlock()
}

type countExporter struct {
	sync.Mutex
	count int64
}

func (e *countExporter) ExportView(vd *Data) {
	if len(vd.Rows) == 0 {
		return
	}
	d := vd.Rows[0].Data.(*CountData)

	e.Lock()
	defer e.Unlock()
	e.count = int64(*d)
}

type vdExporter struct {
	sync.Mutex
	vds []*Data
}

func (e *vdExporter) ExportView(vd *Data) {
	e.Lock()
	defer e.Unlock()

	e.vds = append(e.vds, vd)
}

// restart stops the current processors and creates a new one.
func restart() {
	defaultWorker.stop()
	defaultWorker = newWorker()
	go defaultWorker.start()
}
