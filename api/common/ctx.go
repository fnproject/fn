package common

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
)

type contextKey string

// RequestIDContextKey is the name of the key used to store the request ID into the context
const RequestIDContextKey = "fn_request_id"

// WithRequestID stores a request ID into the context
func WithRequestID(ctx context.Context, rid string) context.Context {
	return context.WithValue(ctx, contextKey(RequestIDContextKey), rid)
}

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

// contextWithNoDeadline is an implementation of context.Context which delegates
// Value() to its parent, but it has no deadline and it is never cancelled, just
// like a context.Background().
type contextWithNoDeadline struct {
	original context.Context
}

func (ctx *contextWithNoDeadline) Deadline() (deadline time.Time, ok bool) {
	return context.Background().Deadline()
}

func (ctx *contextWithNoDeadline) Done() <-chan struct{} {
	return context.Background().Done()
}

func (ctx *contextWithNoDeadline) Err() error {
	return context.Background().Err()
}

func (ctx *contextWithNoDeadline) Value(key interface{}) interface{} {
	return ctx.original.Value(key)
}

// BackgroundContext returns a context that is specifically not a child of the
// provided parent context wrt any cancellation or deadline of the parent,
// so that it contains all values only.
func BackgroundContext(ctx context.Context) context.Context {
	return &contextWithNoDeadline{
		original: ctx,
	}
}
