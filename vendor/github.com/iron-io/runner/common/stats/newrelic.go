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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
)

type NewRelicAgentConfig struct {
	Host    string `json:"host"`
	Version string `json:"version"`
	Pid     int    `json:"pid"`
}

// examples: https://docs.newrelic.com/docs/plugins/plugin-developer-resources/developer-reference/metric-data-plugin-api#examples
type newRelicRequest struct {
	Agent      *agent       `json:"agent"`
	Components []*component `json:"components"`
}

type NewRelicReporterConfig struct {
	Agent      *NewRelicAgentConfig
	LicenseKey string `json:"license_key"`
}

type NewRelicReporter struct {
	Agent      *agent
	LicenseKey string
}

func NewNewRelicReporter(version string, licenseKey string) *NewRelicReporter {
	r := &NewRelicReporter{}
	r.Agent = newNewRelicAgent(version)
	r.LicenseKey = licenseKey
	return r
}

func (r *NewRelicReporter) report(stats []*collectedStat) {
	client := &http.Client{}
	req := &newRelicRequest{}
	req.Agent = r.Agent
	comp := newComponent()
	comp.Name = "IronMQ"
	comp.Duration = 60
	comp.GUID = "io.iron.ironmq"
	// TODO - NR has a fixed 3 level heirarchy? and we just use 2?
	req.Components = []*component{comp}

	// now add metrics
	for _, s := range stats {
		for k, v := range s.Counters {
			comp.Metrics[fmt.Sprintf("Component/%s %s", s.Name, k)] = v
		}
		for k, v := range s.Values {
			comp.Metrics[fmt.Sprintf("Component/%s %s", s.Name, k)] = int64(v)
		}
		for k, v := range s.Timers {
			comp.Metrics[fmt.Sprintf("Component/%s %s", s.Name, k)] = int64(v)
		}
	}

	metricsJson, err := json.Marshal(req)
	if err != nil {
		logrus.WithError(err).Error("error encoding json for NewRelicReporter")
	}

	jsonAsString := string(metricsJson)

	httpRequest, err := http.NewRequest("POST",
		"https://platform-api.newrelic.com/platform/v1/metrics",
		strings.NewReader(jsonAsString))
	if err != nil {
		logrus.WithError(err).Error("error creating New Relic request")
		return
	}
	httpRequest.Header.Set("X-License-Key", r.LicenseKey)
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json")
	httpResponse, err := client.Do(httpRequest)
	if err != nil {
		logrus.WithError(err).Error("error sending http request in NewRelicReporter")
		return
	}
	defer httpResponse.Body.Close()
	body, err := ioutil.ReadAll(httpResponse.Body)
	if err != nil {
		logrus.WithError(err).Error("error reading response body")
	} else {
		logrus.Debugln("response", "code", httpResponse.Status, "body", string(body))
	}
}

type agent struct {
	Host    string `json:"host"`
	Version string `json:"version"`
	Pid     int    `json:"pid"`
}

func newNewRelicAgent(Version string) *agent {
	var err error
	agent := &agent{
		Version: Version,
	}
	agent.Pid = os.Getpid()
	if agent.Host, err = os.Hostname(); err != nil {
		logrus.WithError(err).Error("Can not get hostname")
		return nil
	}
	return agent
}

type component struct {
	Name     string           `json:"name"`
	GUID     string           `json:"guid"`
	Duration int              `json:"duration"`
	Metrics  map[string]int64 `json:"metrics"`
}

func newComponent() *component {
	c := &component{}
	c.Metrics = make(map[string]int64)
	return c
}
