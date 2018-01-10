package server

import (
	"context"
	"os"
	"os/signal"
	"strconv"

	"github.com/fnproject/fn/api/common"
	"github.com/gin-gonic/gin"
)

func init() {
	// gin is not nice by default, this can get set in logging initialization
	gin.SetMode(gin.ReleaseMode)
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
				common.Logger(ctx).Info("Halting...")
				halt()
				return
			case <-ctx.Done():
				common.Logger(ctx).Info("Halting... Original server context canceled.")
				halt()
				return
			}
		}
	}()
	return newCTX, halt
}
