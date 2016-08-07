/*

For keeping a minimum running, perhaps when doing a routing table update, if destination hosts are all
 expired or about to expire we start more.

*/

package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/iron-io/functions/api/config"
	"github.com/iron-io/functions/api/datastore"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/functions/api/server"
	"github.com/spf13/viper"
)

func main() {
	c := &models.Config{}

	config.InitConfig()

	err := c.Validate()
	if err != nil {
		log.WithError(err).Fatalln("Invalid config.")
	}

	ds, err := datastore.New(viper.GetString("db"))
	if err != nil {
		log.WithError(err).Fatalln("Invalid DB url.")
	}

	srv := server.New(ds, c)
	srv.Run()
}
