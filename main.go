/*

For keeping a minimum running, perhaps when doing a routing table update, if destination hosts are all
 expired or about to expire we start more.

*/

package main

import (
	"fmt"
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/iron-io/functions/api/datastore"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/functions/api/server"
	"github.com/spf13/viper"
)

func main() {
	config := &models.Config{}

	InitConfig()
	logLevel, err := log.ParseLevel(viper.GetString("log_level"))
	if err != nil {
		log.WithError(err).Fatalln("Invalid log level.")
	}
	log.SetLevel(logLevel)

	err = config.Validate()
	if err != nil {
		log.WithError(err).Fatalln("Invalid config.")
	}

	ds, err := datastore.New(viper.GetString("db"))
	if err != nil {
		log.WithError(err).Fatalln("Invalid DB url.")
	}

	srv := server.New(ds, config)
	srv.Run()
}

func InitConfig() {
	cwd, _ := os.Getwd()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.SetDefault("log_level", "info")
	viper.SetDefault("db", fmt.Sprintf("bolt://%s/bolt.db?bucket=funcs", cwd))
	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	viper.AutomaticEnv() // picks up env vars automatically
	viper.ReadInConfig()
}
