package server

import (
	"context"
	"os"
	"os/signal"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func init() {
	logLevel, err := logrus.ParseLevel(getEnv(EnvLogLevel, DefaultLogLevel))
	if err != nil {
		logrus.WithError(err).Fatalln("Invalid log level.")
	}
	logrus.SetLevel(logLevel)

	gin.SetMode(gin.ReleaseMode)
	if logLevel == logrus.DebugLevel {
		gin.SetMode(gin.DebugMode)
	}

	// do this in init so that it's only run once & before server.New() which may
	// start things that use spans, which are global.
	// TODO there's not a great reason that our fn spans don't work w/ noop spans, should fix this really.
	setupTracer(getEnv(EnvZipkinURL, ""))
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		// linter liked this better than if/else
		var err error
		var i int
		if i, err = strconv.Atoi(value); err != nil {
			panic(err) // not sure how to handle this
		}
		return i
	}
	return fallback
}

func contextWithSignal(ctx context.Context, signals ...os.Signal) (context.Context, context.CancelFunc) {
	newCTX, halt := context.WithCancel(ctx)
	c := make(chan os.Signal, 1)
	signal.Notify(c, signals...)
	go func() {
		for {
			select {
			case <-c:
				logrus.Info("Halting...")
				halt()
				return
			case <-ctx.Done():
				logrus.Info("Halting... Original server context canceled.")
				halt()
				return
			}
		}
	}()
	return newCTX, halt
}
