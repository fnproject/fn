package main

import (
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
	"golang.org/x/net/context"
)

func init() {
	cwd, err := os.Getwd()
	if err != nil {
		log.WithError(err)
	}
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.SetDefault("log_level", "info")
	viper.SetDefault("mq", fmt.Sprintf("bolt://%s/data/worker_mq.db", cwd))
	viper.SetDefault("db", fmt.Sprintf("bolt://%s/data/bolt.db?bucket=funcs", cwd))
	viper.SetDefault("port", 8080)
	viper.SetDefault("tasks_url", fmt.Sprintf("http://localhost:%d", viper.GetInt("port")))
	viper.SetDefault("nasync", 1)
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

	ds, err := datastore.New(viper.GetString("DB"))
	if err != nil {
		log.WithError(err).Fatalln("Invalid DB url.")
	}
	mqType, err := mqs.New(viper.GetString("MQ"))
	if err != nil {
		log.WithError(err).Fatal("Error on init MQ")
	}
	metricLogger := runner.NewMetricLogger()

	rnr, err := runner.New(metricLogger)
	if err != nil {
		log.WithError(err).Fatalln("Failed to create a runner")
	}

	tasksURL, port, nasync := viper.GetString("tasks_url"), viper.GetString("port"), viper.GetInt("nasync")
	log.Info("async workers:", nasync)
	var wgAsync sync.WaitGroup
	if nasync > 0 {
		wgAsync.Add(1)
		go runner.RunAsyncRunner(ctx, &wgAsync, tasksURL, port, nasync)
	}

	srv := server.New(ds, mqType, rnr)
	go srv.Run(ctx)
	<-ctx.Done()
	wgAsync.Wait()
}
