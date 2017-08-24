package stats

import (
	"time"

	"github.com/sirupsen/logrus"
)

type LogReporter struct {
}

func NewLogReporter() *LogReporter {
	return (&LogReporter{})
}

func (lr *LogReporter) report(stats []*collectedStat) {
	for _, s := range stats {
		f := make(logrus.Fields)
		for k, v := range s.Counters {
			f[k] = v
		}
		for k, v := range s.Values {
			f[k] = v
		}
		for k, v := range s.Timers {
			f[k] = time.Duration(v)
		}

		logrus.WithFields(f).Info(s.Name)
	}
}
