package main

import (
	"context"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/iron-io/functions/api/datastore"
	"github.com/iron-io/functions/api/mqs"
	"github.com/iron-io/functions/api/server"
	"github.com/spf13/viper"
)

func main() {
	ctx := contextWithSignal(context.Background(), os.Interrupt)

	ds, err := datastore.New(viper.GetString(server.EnvDBURL))
	if err != nil {
		log.WithError(err).Fatalln("Invalid DB url.")
	}

	mq, err := mqs.New(viper.GetString(server.EnvMQURL))
	if err != nil {
		log.WithError(err).Fatal("Error on init MQ")
	}

	apiURL := viper.GetString(server.EnvAPIURL)

	funcServer := server.New(ctx, ds, mq, apiURL)
	// Setup your custom extensions, listeners, etc here
	funcServer.Start(ctx)
}
