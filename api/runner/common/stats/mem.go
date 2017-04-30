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
	"runtime"
	"time"
)

func StartReportingMemoryAndGC(reporter Statter, d time.Duration) {
	ticker := time.Tick(d)
	for {
		select {
		case <-ticker:
			var ms runtime.MemStats
			runtime.ReadMemStats(&ms)

			prefix := "runtime"

			reporter.Measure(prefix, "allocated", int64(ms.Alloc), 1.0)
			reporter.Measure(prefix, "allocated.heap", int64(ms.HeapAlloc), 1.0)
			reporter.Time(prefix, "gc.pause", time.Duration(ms.PauseNs[(ms.NumGC+255)%256]), 1.0)

			// GC CPU percentage.
			reporter.Measure(prefix, "gc.cpufraction", int64(ms.GCCPUFraction*100), 1.0)
		}
	}
}
