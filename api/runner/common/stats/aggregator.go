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
	"sync"
	"time"
)

type reporter interface {
	report([]*collectedStat)
}

type collectedStat struct {
	Name     string
	Counters map[string]int64
	Values   map[string]float64
	Gauges   map[string]int64
	Timers   map[string]time.Duration

	avgCounts map[string]uint64
}

func newCollectedStatUnescaped(name string) *collectedStat {
	return &collectedStat{
		Name:      name,
		Counters:  map[string]int64{},
		Values:    map[string]float64{},
		Gauges:    map[string]int64{},
		Timers:    map[string]time.Duration{},
		avgCounts: map[string]uint64{},
	}
}

// What do you call an alligator in a vest?

// Aggregator collects a stats and merges them together if they've been added
// previously. Useful for reporters that have low throughput ie stathat.
type Aggregator struct {
	// Holds all of our stats based on stat.Name
	sl    sync.RWMutex
	stats map[string]*statHolder

	reporters []reporter
}

func newAggregator(reporters []reporter) *Aggregator {
	return &Aggregator{
		stats:     make(map[string]*statHolder),
		reporters: reporters,
	}
}

type statHolder struct {
	cl sync.RWMutex // Lock on Counters
	vl sync.RWMutex // Lock on Values
	s  *collectedStat
}

func newStatHolder(st *collectedStat) *statHolder {
	return &statHolder{s: st}
}

type kind int16

const (
	counterKind kind = iota
	valueKind
	gaugeKind
	durationKind
)

func (a *Aggregator) add(component, key string, kind kind, value interface{}) {
	a.sl.RLock()
	stat, ok := a.stats[component]
	a.sl.RUnlock()
	if !ok {
		a.sl.Lock()
		stat, ok = a.stats[component]
		if !ok {
			stat = newStatHolder(newCollectedStatUnescaped(component))
			a.stats[component] = stat
		}
		a.sl.Unlock()
	}

	if kind == counterKind || kind == gaugeKind {
		var mapPtr map[string]int64
		if kind == counterKind {
			mapPtr = stat.s.Counters
		} else {
			mapPtr = stat.s.Gauges
		}
		value := value.(int64)
		stat.cl.Lock()
		mapPtr[key] += value
		stat.cl.Unlock()
	}

	/* TODO: this ends up ignoring tags so yeah gg
	/  lets just calculate a running average for now. Can do percentiles later
	/  Recalculated Average
	/
	/     currentAverage * currentCount + newValue
	/    ------------------------------------------
	/                (currentCount +1)
	/
	*/
	if kind == valueKind || kind == durationKind {
		var typedValue int64
		if kind == valueKind {
			typedValue = value.(int64)
		} else {
			typedValue = int64(value.(time.Duration))
		}

		stat.vl.Lock()
		switch kind {
		case valueKind:
			oldAverage := stat.s.Values[key]
			count := stat.s.avgCounts[key]
			newAverage := (oldAverage*float64(count) + float64(typedValue)) / (float64(count + 1))
			stat.s.avgCounts[key] = count + 1
			stat.s.Values[key] = newAverage
		case durationKind:
			oldAverage := float64(stat.s.Timers[key])
			count := stat.s.avgCounts[key]
			newAverage := (oldAverage*float64(count) + float64(typedValue)) / (float64(count + 1))
			stat.s.avgCounts[key] = count + 1
			stat.s.Timers[key] = time.Duration(newAverage)
		}
		stat.vl.Unlock()
	}
}

func (a *Aggregator) dump() []*collectedStat {
	a.sl.Lock()
	bucket := a.stats
	// Clear out the maps, effectively resetting our average
	a.stats = make(map[string]*statHolder)
	a.sl.Unlock()

	stats := make([]*collectedStat, 0, len(bucket))
	for _, v := range bucket {
		stats = append(stats, v.s)
	}
	return stats
}

func (a *Aggregator) report(st []*collectedStat) {
	stats := a.dump()
	stats = append(stats, st...)
	for _, r := range a.reporters {
		r.report(stats)
	}
}

func (r *Aggregator) Inc(component string, stat string, value int64, rate float32) {
	r.add(component, stat, counterKind, value)
}

func (r *Aggregator) Gauge(component string, stat string, value int64, rate float32) {
	r.add(component, stat, gaugeKind, value)
}

func (r *Aggregator) Measure(component string, stat string, value int64, rate float32) {
	r.add(component, stat, valueKind, value)
}

func (r *Aggregator) Time(component string, stat string, value time.Duration, rate float32) {
	r.add(component, stat, durationKind, value)
}

func (r *Aggregator) NewTimer(component string, stat string, rate float32) *Timer {
	return newTimer(r, component, stat, rate)
}
