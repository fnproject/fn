package runner

import (
	"context"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/iron-io/runner/common"
)

type MetricLogger interface {
	Log(context.Context, map[string]interface{})
	LogCount(context.Context, string, int)
	LogGauge(context.Context, string, int)
	LogTime(context.Context, string, time.Duration)
}

type Metric map[string]interface{}

func NewMetricLogger() MetricLogger {
	return &DefaultMetricLogger{}
}

type DefaultMetricLogger struct{}

func (l *DefaultMetricLogger) Log(ctx context.Context, metric map[string]interface{}) {
	log := common.Logger(ctx)
	log.WithFields(logrus.Fields(metric)).Info()
}

func (l *DefaultMetricLogger) LogCount(ctx context.Context, name string, value int) {
	l.Log(ctx, Metric{
		"name":  name,
		"value": value,
		"type":  "count",
	})
}

func (l *DefaultMetricLogger) LogTime(ctx context.Context, name string, value time.Duration) {
	l.Log(ctx, Metric{
		"name":  name,
		"value": value,
		"type":  "time",
	})
}

func (l *DefaultMetricLogger) LogGauge(ctx context.Context, name string, value int) {
	l.Log(ctx, Metric{
		"name":  name,
		"value": value,
		"type":  "gauge",
	})
}
