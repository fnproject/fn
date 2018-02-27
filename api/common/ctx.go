package common

import (
	"context"

	"github.com/sirupsen/logrus"
	"go.opencensus.io/tag"
	"go.opencensus.io/trace"
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

// LoggerWithFields returns a child context of the provided parent that
// contains a logger with additional fields from the parent's logger, it
// returns the new child logger, as well.
func LoggerWithFields(ctx context.Context, fields logrus.Fields) (context.Context, logrus.FieldLogger) {
	l := Logger(ctx)
	l = l.WithFields(fields)
	ctx = WithLogger(ctx, l)
	return ctx, l
}

// BackgroundContext returns a context that is specifically not a child of the
// provided parent context wrt any cancellation or deadline of the parent,
// returning a context that contains all values only. At present, this is a
// best effort as there is not a great way to extract all values, known values:
// * logger
// * span
// * tags
// (TODO(reed): we could have our own context.Context implementer that stores
// all values from WithValue in a bucket we could extract more easily?)
func BackgroundContext(ctx context.Context) context.Context {
	logger := Logger(ctx)
	span := trace.FromContext(ctx)
	tagMap := tag.FromContext(ctx)

	// fresh context
	ctx = context.Background()

	ctx = tag.NewContext(ctx, tagMap)
	ctx = trace.WithSpan(ctx, span)
	ctx = WithLogger(ctx, logger)
	return ctx
}
