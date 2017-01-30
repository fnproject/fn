package server

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
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
	viper.SetDefault(EnvDBURL, fmt.Sprintf("bolt://%s/data/bolt.db?bucket=funcs", cwd))
	viper.SetDefault(EnvPort, 8080)
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

func contextWithSignal(ctx context.Context, signals ...os.Signal) context.Context {
	ctx, halt := context.WithCancel(context.Background())
	c := make(chan os.Signal, 1)
	signal.Notify(c, signals...)
	go func() {
		<-c
		logrus.Info("Halting...")
		halt()
	}()
	return ctx
}
