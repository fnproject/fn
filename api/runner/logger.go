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
	"bufio"
	"io"

	"github.com/Sirupsen/logrus"
)

// FuncLogger reads STDERR output from a container and outputs it in a parseable structured log format, see: https://github.com/iron-io/functions/issues/76
type FuncLogger struct {
	r io.Reader
	w io.Writer
}

func NewFuncLogger(appName, path, function, requestID string) io.Writer {
	r, w := io.Pipe()
	funcLogger := &FuncLogger{
		r: r,
		w: w,
	}
	log := logrus.WithFields(logrus.Fields{"user_log": true, "app_name": appName, "path": path, "function": function, "call_id": requestID})
	go func(reader io.Reader) {
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			log.Println(scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			log.WithError(err).Println("There was an error with the scanner in attached container")
		}
	}(r)
	return funcLogger
}

func (l *FuncLogger) Write(p []byte) (n int, err error) {
	return l.w.Write(p)
}
