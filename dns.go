package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/iron-io/go/common"
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
func registerHost(w http.ResponseWriter, r *http.Request, app *App) bool {
	// Give it an iron.computer entry with format:
	// WORKER_NAME.PROJECT_ID.iron.computer.
	dnsHost := fmt.Sprintf("%v.%v.iron.computer", app.Name, 123)
	app.Dns = dnsHost
	log15.Info("registering dns", "dnsname", dnsHost)

	if app.CloudFlareId == "" {
		// Tad hacky, but their go lib is pretty confusing.
		cfMethod := "POST"
		cfUrl := "https://api.cloudflare.com/client/v4/zones/29a42a6c6b9b2ed4b843b78d65b8af89/dns_records"
		if app.CloudFlareId != "" {
			// Have this here in case we need to support updating the entry. If we do this, is how:
			cfMethod = "PUT"
			cfUrl = cfUrl + "/" + app.CloudFlareId
		}
		cfbody := "{\"type\":\"CNAME\",\"name\":\"" + dnsHost + "\",\"content\":\"router.iron.computer\",\"ttl\":120}"
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
		app.CloudFlareId = cfResult.Result.Id
	}

	log15.Info("host registered successfully with cloudflare", "app", app)
	return true
}
