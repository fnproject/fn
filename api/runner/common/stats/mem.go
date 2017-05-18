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
