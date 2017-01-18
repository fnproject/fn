package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/server"
	"github.com/spf13/viper"
)

func init() {
	viper.AutomaticEnv() // picks up env vars automatically
	cwd, err := os.Getwd()
	if err != nil {
		logrus.WithError(err).Fatalln("")
	}
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.SetDefault(server.EnvLogLevel, "info")
	viper.SetDefault(server.EnvMQURL, fmt.Sprintf("bolt://%s/data/worker_mq.db", cwd))
	viper.SetDefault(server.EnvDBURL, fmt.Sprintf("bolt://%s/data/bolt.db?bucket=funcs", cwd))
	viper.SetDefault(server.EnvPort, 8080)
	viper.SetDefault(server.EnvAPIURL, fmt.Sprintf("http://127.0.0.1:%d", viper.GetInt(server.EnvPort)))
	logLevel, err := logrus.ParseLevel(viper.GetString(server.EnvLogLevel))
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
