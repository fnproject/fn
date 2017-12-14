package common

import (
	"context"

	"github.com/sirupsen/logrus"
)

type contextKey string

// WithLogger stores the logger.
func WithLogger(ctx context.Context, l logrus.FieldLogger) context.Context {
	return context.WithValue(ctx, contextKey("logger"), l)
}

// Logger returns the structured logger.
func Logger(ctx context.Context) logrus.FieldLogger {
	l, ok := ctx.Value(contextKey("logger")).(logrus.FieldLogger)
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
