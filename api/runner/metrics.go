package runner

import (
	"github.com/Sirupsen/logrus"
	titancommon "github.com/iron-io/titan/common"
	"golang.org/x/net/context"
)

type Logger interface {
	Log(context.Context, map[string]interface{})
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
