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
	"time"

	"github.com/Sirupsen/logrus"
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
