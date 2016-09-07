package runner

import (
	"time"

	"github.com/Sirupsen/logrus"
	titancommon "github.com/iron-io/titan/common"
	"golang.org/x/net/context"
)

func LogMetric(ctx context.Context, name string, metricType string, value interface{}) {
	log := titancommon.Logger(ctx)
	log.WithFields(logrus.Fields{
		"metric": name, "type": metricType, "value": value}).Info()
}

func LogMetricGauge(ctx context.Context, name string, value int) {
	LogMetric(ctx, name, "gauge", value)
}

func LogMetricCount(ctx context.Context, name string, value int) {
	LogMetric(ctx, name, "count", value)
}

func LogMetricTime(ctx context.Context, name string, time time.Duration) {
	LogMetric(ctx, name, "time", time)
}
