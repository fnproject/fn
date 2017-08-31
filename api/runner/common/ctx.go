package common

import (
	"context"

	"github.com/sirupsen/logrus"
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

// LoggerWithStack is a helper to add a stack trace to the logs. call is typically the name of the current function.
func LoggerWithStack(ctx context.Context, call string) (context.Context, logrus.FieldLogger) {
	l := Logger(ctx)
	entry, ok := l.(*logrus.Entry)
	if !ok {
		// probably a StandardLogger with no "stack" entry yet
		l = l.WithField("stack", call)
		ctx = WithLogger(ctx, l)
		return ctx, l
	}
	// grab the stack field and append to it
	v, ok := entry.Data["stack"]
	stack := ""
	if ok && v != nil {
		stack = v.(string)
	}
	stack += "->" + call
	l = l.WithField("stack", stack)
	ctx = WithLogger(ctx, l)
	return ctx, l
}
