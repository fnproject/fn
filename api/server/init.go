package server

import (
	"context"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

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
	} else if value, ok := os.LookupEnv(key + "_FILE"); ok {
		dat, err := ioutil.ReadFile(filepath.Clean(value))
		if err == nil {
			return string(dat)
		}
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		// linter liked this better than if/else
		var err error
		var i int
		if i, err = strconv.Atoi(value); err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{"string": value, "environment_key": key}).Fatal("Failed to convert string to int")
		}
		return i
	} else if value, ok := os.LookupEnv(key + "_FILE"); ok {
		dat, err := ioutil.ReadFile(filepath.Clean(value))
		if err == nil {
			var err error
			var i int
			if i, err = strconv.Atoi(strings.TrimSpace(string(dat))); err != nil {
				logrus.WithError(err).WithFields(logrus.Fields{"string": dat, "environment_key": key}).Fatal("Failed to convert string to int")
			}
			return i
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	var err error
	res := fallback
	if tmp := os.Getenv(key); tmp != "" {
		res, err = time.ParseDuration(tmp + "s")
		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{"duration_string": tmp, "environment_key": key}).Fatal("Failed to parse duration from environment")
		}
	}
	return res
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
