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
	"net/http"
	"net/url"
	"strconv"

	"github.com/Sirupsen/logrus"
)

func postStatHat(key, stat string, values url.Values) {
	values.Set("stat", stat)
	values.Set("ezkey", key)
	resp, err := http.PostForm("http://api.stathat.com/ez", values)
	if err != nil {
		logrus.WithError(err).Error("couldn't post to StatHat")
		return
	}
	if resp.StatusCode != 200 {
		logrus.Errorln("bad status posting to StatHat", "status_code", resp.StatusCode)
	}
	resp.Body.Close()
}

type StatHatReporterConfig struct {
	Email  string
	Prefix string
}

func (shr *StatHatReporterConfig) report(stats []*collectedStat) {
	for _, s := range stats {
		for k, v := range s.Counters {
			n := shr.Prefix + " " + s.Name + " " + k
			values := url.Values{}
			values.Set("count", strconv.FormatInt(v, 10))
			postStatHat(shr.Email, n, values)
		}
		for k, v := range s.Values {
			n := shr.Prefix + " " + s.Name + " " + k
			values := url.Values{}
			values.Set("value", strconv.FormatFloat(v, 'f', 3, 64))
			postStatHat(shr.Email, n, values)
		}
		for k, v := range s.Timers {
			n := shr.Prefix + " " + s.Name + " " + k
			values := url.Values{}
			values.Set("value", strconv.FormatInt(int64(v), 10))
			postStatHat(shr.Email, n, values)
		}
	}
}
