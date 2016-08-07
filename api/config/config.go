package config

import (
	"fmt"
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/spf13/viper"
)

func InitConfig() {
	cwd, _ := os.Getwd()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.SetDefault("log_level", "info")
	viper.SetDefault("db", fmt.Sprintf("bolt://%s/data/bolt.db?bucket=funcs", cwd))
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
