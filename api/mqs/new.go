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

package mqs

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/iron-io/functions/api/models"
)

// New will parse the URL and return the correct MQ implementation.
func New(mqURL string) (models.MessageQueue, error) {
	// Play with URL schemes here: https://play.golang.org/p/xWAf9SpCBW
	u, err := url.Parse(mqURL)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{"url": mqURL}).Fatal("bad MQ URL")
	}
	logrus.WithFields(logrus.Fields{"mq": u.Scheme}).Debug("selecting MQ")
	switch u.Scheme {
	case "memory":
		return NewMemoryMQ(), nil
	case "redis":
		return NewRedisMQ(u)
	case "bolt":
		return NewBoltMQ(u)
	}
	if strings.HasPrefix(u.Scheme, "ironmq") {
		return NewIronMQ(u), nil
	}

	return nil, fmt.Errorf("mq type not supported %v", u.Scheme)
}
