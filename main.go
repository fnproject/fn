package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/iron-io/functions/api/datastore"
	"github.com/iron-io/functions/api/mqs"
	"github.com/iron-io/functions/api/runner"
	"github.com/iron-io/functions/api/server"
	"github.com/spf13/viper"
)

const (
	envLogLevel = "log_level"
	envMQ       = "mq"
	envDB       = "db"
	envPort     = "port" // be careful, Gin expects this variable to be "port"
	envAPIURL   = "api_url"
	envNumAsync = "num_async"
)

func init() {
	cwd, err := os.Getwd()
	if err != nil {
		log.WithError(err)
	}
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.SetDefault(envLogLevel, "info")
	viper.SetDefault(envMQ, fmt.Sprintf("bolt://%s/data/worker_mq.db", cwd))
	viper.SetDefault(envDB, fmt.Sprintf("bolt://%s/data/bolt.db?bucket=funcs", cwd))
	viper.SetDefault(envPort, 8080)
	viper.SetDefault(envAPIURL, fmt.Sprintf("http://127.0.0.1:%d", viper.GetInt(envPort)))
	viper.SetDefault(envNumAsync, 1)
	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	viper.AutomaticEnv() // picks up env vars automatically
	viper.ReadInConfig()
	logLevel, err := log.ParseLevel(viper.GetString("log_level"))
	if err != nil {
		log.WithError(err).Fatalln("Invalid log level.")
	}
	log.SetLevel(logLevel)
}

func main() {
	ctx, halt := context.WithCancel(context.Background())
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		log.Info("Halting...")
		halt()
	}()

	ds, err := datastore.New(viper.GetString(envDB))
	if err != nil {
		log.WithError(err).Fatalln("Invalid DB url.")
	}
	mqType, err := mqs.New(viper.GetString(envMQ))
	if err != nil {
		log.WithError(err).Fatal("Error on init MQ")
	}
	metricLogger := runner.NewMetricLogger()

	rnr, err := runner.New(metricLogger)
	if err != nil {
		log.WithError(err).Fatalln("Failed to create a runner")
	}

	apiURL, port, numAsync := viper.GetString(envAPIURL), viper.GetString(envPort), viper.GetInt(envNumAsync)
	log.Info("async workers:", numAsync)
	var wgAsync sync.WaitGroup
	if numAsync > 0 {
		wgAsync.Add(1)
		go runner.RunAsyncRunner(ctx, &wgAsync, apiURL, port, numAsync)
	}

	srv := server.New(ds, mqType, rnr)
	go srv.Run(ctx)
	<-ctx.Done()
	wgAsync.Wait()
}
