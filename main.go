package main

import (
	"fmt"
	"strconv"
	"strings"

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

	mqType, err := mqs.New(viper.GetString("MQ"))
	if err != nil {
		log.WithError(err).Fatal("Error on init MQ")
	}

	mqAdr := strings.TrimSpace(viper.GetString("MQADR"))
	port := viper.GetInt("PORT")
	if port == 0 {
		port = 8080
	}
	if mqAdr == "" {
		mqAdr = fmt.Sprintf("localhost:%d", port)
	}

	metricLogger := runner.NewMetricLogger()

	rnr, err := runner.New(metricLogger)
	if err != nil {
		log.WithError(err).Fatalln("Failed to create a runner")
	}

	srv := server.New(ds, mqType, rnr)
	go srv.Run(ctx)

	nasync := 1
	if nasyncStr := strings.TrimSpace(viper.GetString("NASYNC")); len(nasyncStr) > 0 {
		var err error
		nasync, err = strconv.Atoi(nasyncStr)
		if err != nil {
			log.WithError(err).Fatalln("Failed to parse number of async runners")
		}
	}

	for i := 0; i < nasync; i++ {
		go runner.RunAsyncRunners(mqAdr)
	}

	quit := make(chan bool)
	for _ = range quit {
	}
}
