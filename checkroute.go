
package main

import (
	"github.com/iron-io/iron_go/cache"
	"github.com/iron-io/common"

	"encoding/json"
	"log"
)

var config struct {
	Iron struct {
	Token      string `json:"token"`
	ProjectId  string `json:"project_id"`
	SuperToken string `json:"super_token"`
	Host 	   string `json:"host"`
} `json:"iron"`
	MongoAuth common.MongoConfig `json:"mongo_auth"`
	Logging   struct {
	To     string `json:"to"`
	Level  string `json:"level"`
	Prefix string `json:"prefix"`
}
}


var icache = cache.New("routing-table")

func main(){
	log.Println("STARTING")

	var configFile string
	var env string
	env = "development"
	configFile = "config_" + env + ".json"
	common.LoadConfig("iron_mq", configFile, &config)
	common.SetLogLevel(config.Logging.Level)
//	common.SetLogLocation(config.Logging.To, config.Logging.Prefix)

	host := "routertest.irondns.info"
	log.Println("CHECKING ROUTE")

	route, err := getRoute(host)
	log.Println("route:", route)
	log.Println("err:", err)


}


type Route struct {
	// TODO: Change destinations to a simple cache so it can expire entries after 55 minutes (the one we use in common?)
	Host         string   `json:"host"`
	Destinations []string `json:"destinations"`
	ProjectId    string   `json:"project_id"`
	Token        string   `json:"token"` // store this so we can queue up new workers on demand
	CodeName     string   `json:"code_name"`
}


func getRoute(host string) (*Route, error) {
	rx, err := icache.Get(host)
	if err != nil {
		return nil, err
	}
	rx2 := []byte(rx.(string))
	route := Route{}
	err = json.Unmarshal(rx2, &route)
	if err != nil {
		return nil, err
	}
	return &route, err
}
