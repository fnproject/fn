package server

import (
	"context"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"

	"github.com/fnproject/fn/api/common"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
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

// Installs a child process reaper if init process
func installChildReaper() {
	// assume responsibilities of init process if running as init process for Linux
	if runtime.GOOS != "linux" || os.Getpid() != 1 {
		return
	}

	var sigs = make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGCHLD)

	// we run this forever and leak a go routine. As init, we must
	// reap our children until the very end, so this is OK.
	go func() {
		for {
			<-sigs
			for {
				var status syscall.WaitStatus
				var rusage syscall.Rusage

				pid, err := syscall.Wait4(-1, &status, syscall.WNOHANG, &rusage)
				// no children
				if pid <= 0 {
					break
				}

				logrus.Infof("Child terminated pid=%d err=%v status=%v usage=%v", pid, err, status, rusage)
			}
		}
	}()
}
