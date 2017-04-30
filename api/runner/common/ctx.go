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

package common

import (
	"context"

	"github.com/Sirupsen/logrus"
)

// WithLogger stores the logger.
func WithLogger(ctx context.Context, l logrus.FieldLogger) context.Context {
	return context.WithValue(ctx, "logger", l)
}

// Logger returns the structured logger.
func Logger(ctx context.Context) logrus.FieldLogger {
	l, ok := ctx.Value("logger").(logrus.FieldLogger)
	if !ok {
		return logrus.StandardLogger()
	}
	return l
}

// Attempt at simplifying this whole logger in the context thing
// Could even make this take a generic map, then the logger that gets returned could be used just like the stdlib too, since it's compatible
func LoggerWithFields(ctx context.Context, fields logrus.Fields) (context.Context, logrus.FieldLogger) {
	l := Logger(ctx)
	l = l.WithFields(fields)
	ctx = WithLogger(ctx, l)
	return ctx, l
}
