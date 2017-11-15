package server

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func init() {
	viper.AutomaticEnv() // picks up env vars automatically
	cwd, err := os.Getwd()
	if err != nil {
		logrus.WithError(err).Fatalln("")
	}
	// Replace forward slashes in case this is windows, URL parser errors
	cwd = strings.Replace(cwd, "\\", "/", -1)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.SetDefault(EnvLogLevel, "info")
	viper.SetDefault(EnvMQURL, fmt.Sprintf("bolt://%s/data/worker_mq.db", cwd))
	viper.SetDefault(EnvDBURL, fmt.Sprintf("sqlite3://%s/data/fn.db", cwd))
	viper.SetDefault(EnvLOGDBURL, "") // default to just using DB url
	viper.SetDefault(EnvPort, 8080)
	viper.SetDefault(EnvZipkinURL, "") // off default
	viper.SetDefault(EnvAPIURL, fmt.Sprintf("http://127.0.0.1:%d", viper.GetInt(EnvPort)))
	viper.AutomaticEnv() // picks up env vars automatically
	logLevel, err := logrus.ParseLevel(viper.GetString(EnvLogLevel))
	if err != nil {
		logrus.WithError(err).Fatalln("Invalid log level.")
	}
	logrus.SetLevel(logLevel)

	gin.SetMode(gin.ReleaseMode)
	if logLevel == logrus.DebugLevel {
		gin.SetMode(gin.DebugMode)
	}
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
