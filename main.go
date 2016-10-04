package main

import (
	"fmt"
	"os"
	"strings"

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
	viper.SetDefault("tasksrv", fmt.Sprintf("http://localhost:%d", viper.GetInt("port")))
	viper.SetDefault("NASYNC", 1)
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
	ctx := context.Background()

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

	tasksrv, port := viper.GetString("PORT"), viper.GetString("TASKSVR")
	for nasync, i := viper.GetInt("NASYNC"), 0; i < nasync; i++ {
		go runner.RunAsyncRunner(tasksrv, port)
	}

	srv := server.New(ds, mqType, rnr)
	srv.Run(ctx)
}
