package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/iron-io/go/common"
	"github.com/iron-io/go/swapi/backend"
	"github.com/iron-io/iron_go/cache"
	"gopkg.in/inconshreveable/log15.v2"
)

type CloudFlareResult struct {
	Id string `json:"id"`
}
type CloudFlareResponse struct {
	Result  CloudFlareResult `json:"result"`
	Success bool
}

/*
This function registers the host name provided (dns host) in IronCache which is used for
the routing table. Backend runners know this too.

This will also create a dns entry for the worker in iron.computer.
*/
func registerHost(w http.ResponseWriter, r *http.Request, code *backend.Code) bool {
	dnsHost := fmt.Sprintf("%v.%v.iron.computer", strings.Replace(code.Name, " ", "-", -1), code.ProjectId.Hex())
	log15.Info("registering host", "host", code.Host, "ironhost", dnsHost)

	code.Host = &dnsHost
	route, err := getRoute(code)
	if err == nil {
		if route.ProjectId != code.ProjectId.Hex() {
			// somebody else already register this host
			common.SendError(w, 400, fmt.Sprint("This host is already registered. If you believe this is in error, please contact support@iron.io to resolve the issue.", err))
			return false
		}
	}
	if route == nil {
		route = &backend.Route{
			CodeId:    code.Id.Hex(),
			Host:      dnsHost,
			ProjectId: code.ProjectId.Hex(),
			CodeName:  code.Name,
		}
	}

	if route.CloudFlareId == "" {
		// And give it an iron.computer entry too with format:
		// WORKER_NAME.PROJECT_ID.iron.computer.
		// Tad hacky, but their go lib is pretty confusing.
		cfMethod := "POST"
		cfUrl := "https://api.cloudflare.com/client/v4/zones/29a42a6c6b9b2ed4b843b78d65b8af89/dns_records"
		if route.CloudFlareId != "" {
			// Have this here in case we need to support updating the entry. If we do this, is how:
			cfMethod = "PUT"
			cfUrl = cfUrl + "/" + route.CloudFlareId
		}
		cfbody := "{\"type\":\"CNAME\",\"name\":\"" + dnsHost + "\",\"content\":\"router.iron.io\",\"ttl\":120}"
		client := &http.Client{} // todo: is default client fine?
		req, err := http.NewRequest(
			cfMethod,
			cfUrl,
			strings.NewReader(cfbody),
		)
		req.Header.Set("X-Auth-Email", config.CloudFlare.Email)
		req.Header.Set("X-Auth-Key", config.CloudFlare.AuthKey)
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			log15.Error("Could not register dns entry.", "err", err)
			common.SendError(w, 500, fmt.Sprint("Could not register dns entry.", err))
			return false
		}
		defer resp.Body.Close()
		// todo: get error message from body for bad status code
		body, err := ioutil.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			log15.Error("Could not register dns entry 2.", "code", resp.StatusCode, "body", string(body))
			common.SendError(w, 500, fmt.Sprint("Could not register dns entry 2. ", resp.StatusCode))
			return false
		}
		cfResult := CloudFlareResponse{}
		err = json.Unmarshal(body, &cfResult)
		if err != nil {
			log15.Error("Could not parse DNS response.", "err", err, "code", resp.StatusCode, "body", string(body))
			common.SendError(w, 500, fmt.Sprint("Could not parse DNS response. ", resp.StatusCode))
			return false
		}
		fmt.Println("cfresult:", cfResult)
		route.CloudFlareId = cfResult.Result.Id
	}

	// Now store it
	// todo: do we even need to update the route after the first time??
	err = putRoute(route)
	if err != nil {
		log15.Error("Could not register host.", "err", err)
		common.SendError(w, 500, fmt.Sprint("Could not register host.", err))
		return false
	}

	log15.Info("host registered successfully", "route", route)
	return true
}

func getRoute(code *backend.Code) (*backend.Route, error) {
	log15.Info("in getRoute", "icache_settings", icache.Settings, "key", *code.Host)
	rx, err := icache.Get(*code.Host)
	if err != nil {
		return nil, err
	}
	rx2 := []byte(rx.(string))
	var route backend.Route
	err = json.Unmarshal(rx2, &route)
	if err != nil {
		return nil, err
	}
	return &route, err
}

func putRoute(route *backend.Route) error {
	log15.Info("in putRoute", "icache_settings", icache.Settings)
	item := cache.Item{}
	v, err := json.Marshal(route)
	if err != nil {
		return err
	}
	item.Value = string(v)
	log15.Info("about to put route", "route", route, "host", route.Host)
	// todo: put some kind of reasonable expiry on this (a month or two)
	err = icache.Put(route.Host, &item)
	return err
}
