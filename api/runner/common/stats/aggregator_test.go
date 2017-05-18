package stats

import (
	"fmt"
	"math/rand"
	"testing"
)

func TestAggregator(t *testing.T) {
	ag := newAggregator([]reporter{})
	var sum int64 = 0
	var times int64 = 0
	for i := 0; i < 100; i++ {
		ag.add("mq push", "messages", counterKind, int64(1))
		ag.add("mq push", "latency", valueKind, int64(i))
		ag.add("mq pull", "latency", valueKind, int64(i))
		sum += int64(i)
		times += 1
	}

	for _, stat := range ag.dump() {
		for k, v := range stat.Values {
			if v != float64(sum)/float64(times) {
				t.Error("key:", k, "Expected", sum/times, "got", v)
			}
		}

		for k, v := range stat.Counters {
			if v != times {
				t.Error("key:", k, "Expected", times, "got", v)
			}
		}
	}
	if len(ag.stats) != 0 {
		t.Error("expected stats map to be clear, got", len(ag.stats))
	}
}

type testStat struct {
	component string
	key       string
	kind      kind
	value     int64
}

func BenchmarkAggregatorAdd(b *testing.B) {
	ag := &Aggregator{
		stats: make(map[string]*statHolder, 1000),
	}

	s := createStatList(1000)

	sl := len(s)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			e := s[rand.Intn(sl)]
			ag.add(e.component, e.key, e.kind, e.value)
		}
	})
}

func createStatList(n int) []*testStat {
	var stats []*testStat
	for i := 0; i < n; i++ {
		st := testStat{
			component: "aggregator_test",
			key:       fmt.Sprintf("latency.%d", i),
			kind:      counterKind,
			value:     1,
		}

		if rand.Float32() < 0.5 {
			st.key = fmt.Sprintf("test.%d", i)
			st.kind = valueKind
			st.value = 15999
		}
		stats = append(stats, &st)
	}
	return stats
}
