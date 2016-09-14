package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/iron-io/functions/api/config"
	"github.com/iron-io/functions/api/datastore"
	"github.com/iron-io/functions/api/mqs"
	"github.com/iron-io/functions/api/runner"
	"github.com/iron-io/functions/api/server"
	"github.com/spf13/viper"
	"golang.org/x/net/context"
)

func main() {
	ctx := context.Background()

	config.InitConfig()

	ds, err := datastore.New(viper.GetString("DB"))
	if err != nil {
		log.WithError(err).Fatalln("Invalid DB url.")
	}

	mq, err := mqs.New(viper.GetString("mq"))
	if err != nil {
		log.WithError(err).Fatal("Error on init MQ")
	}

	metricLogger := runner.NewMetricLogger()
	runner, err := runner.New(metricLogger)

	if err != nil {
		log.WithError(err).Fatalln("Failed to create a runner")
	}

	srv := server.New(ds, mq, runner)
	srv.Run(ctx)
}
