package runner

import (
	"time"

	"github.com/Sirupsen/logrus"
	titancommon "github.com/iron-io/worker/common"
	"golang.org/x/net/context"
)

type Logger interface {
	Log(context.Context, map[string]interface{})
	LogCount(context.Context, string, int)
	LogGauge(context.Context, string, int)
	LogTime(context.Context, string, time.Duration)
}

type Metric map[string]interface{}

func NewMetricLogger() *MetricLogger {
	return &MetricLogger{}
}

type MetricLogger struct{}

func (l *MetricLogger) Log(ctx context.Context, metric map[string]interface{}) {
	log := titancommon.Logger(ctx)
	log.WithFields(logrus.Fields(metric)).Info()
}

func (l *MetricLogger) LogCount(ctx context.Context, name string, value int) {
	l.Log(ctx, Metric{
		"name":  name,
		"value": value,
		"type":  "count",
	})
}

func (l *MetricLogger) LogTime(ctx context.Context, name string, value time.Duration) {
	l.Log(ctx, Metric{
		"name":  name,
		"value": value,
		"type":  "time",
	})
}

func (l *MetricLogger) LogGauge(ctx context.Context, name string, value int) {
	l.Log(ctx, Metric{
		"name":  name,
		"value": value,
		"type":  "gauge",
	})
}
