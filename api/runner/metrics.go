package runner

import (
	"time"

	"github.com/Sirupsen/logrus"
	titancommon "github.com/iron-io/titan/common"
	"golang.org/x/net/context"
)

func LogMetricGauge(ctx context.Context, name string, value int) {
	log := titancommon.Logger(ctx)
	log.WithFields(logrus.Fields{
		"metric": name, "type": "count", "value": value}).Info()
}

func LogMetricCount(ctx context.Context, name string, value int) {
	log := titancommon.Logger(ctx)
	log.WithFields(logrus.Fields{
		"metric": name, "type": "count", "value": value}).Info()
}

func LogMetricTime(ctx context.Context, name string, time time.Duration) {
	log := titancommon.Logger(ctx)
	log.WithFields(logrus.Fields{
		"metric": name, "type": "time", "value": time}).Info()
}
