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

package runner

import (
	"context"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/iron-io/runner/common"
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
	log := common.Logger(ctx)
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
