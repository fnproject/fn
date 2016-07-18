/*

For keeping a minimum running, perhaps when doing a routing table update, if destination hosts are all
 expired or about to expire we start more.

*/

package main

import (
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/iron-io/functions/api"
)

func main() {

	config := &api.Config{}
	config.CloudFlare.Email = os.Getenv("CLOUDFLARE_EMAIL")
	config.CloudFlare.ApiKey = os.Getenv("CLOUDFLARE_API_KEY")
	config.CloudFlare.ZoneId = os.Getenv("CLOUDFLARE_ZONE_ID")

	// TODO: validate inputs, iron tokens, cloudflare stuff, etc
	err := config.Validate()
	if err != nil {
		log.WithError(err).Fatalln("Invalid config.")
	}
	log.Printf("config: %+v", config)

	api := api.New(config)
	api.Start()
}
