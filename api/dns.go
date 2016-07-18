package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	log "github.com/Sirupsen/logrus"
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
	// Give it an iron.computer entry with format: APP_NAME.PROJECT_ID.ironfunctions.com
	dnsHost := fmt.Sprintf("%v.%v.ironfunctions.com", app.Name, 123)
	app.Dns = dnsHost

	if app.CloudFlareId == "" {
		// Tad hacky, but their go lib is pretty confusing.
		cfMethod := "POST"
		cfUrl := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%v/dns_records", config.CloudFlare.ZoneId)
		if app.CloudFlareId != "" {
			// Have this here in case we need to support updating the entry. If we do this, is how:
			cfMethod = "PUT"
			cfUrl = cfUrl + "/" + app.CloudFlareId
		}
		log.Info("registering dns: ", "dnsname: ", dnsHost, " url: ", cfUrl)

		cfbody := "{\"type\":\"CNAME\",\"name\":\"" + dnsHost + "\",\"content\":\"api.ironfunctions.com\",\"ttl\":120}"
		client := &http.Client{} // todo: is default client fine?
		req, err := http.NewRequest(
			cfMethod,
			cfUrl,
			strings.NewReader(cfbody),
		)
		req.Header.Set("X-Auth-Email", config.CloudFlare.Email)
		req.Header.Set("X-Auth-Key", config.CloudFlare.ApiKey)
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			log.Error("Could not register dns entry.", "err", err)
			SendError(w, 500, fmt.Sprint("Could not register dns entry.", err))
			return false
		}
		defer resp.Body.Close()
		// todo: get error message from body for bad status code
		body, err := ioutil.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			log.Error("Could not register dns entry 2.", "code", resp.StatusCode, "body", string(body))
			SendError(w, 500, fmt.Sprint("Could not register dns entry 2. ", resp.StatusCode))
			return false
		}
		cfResult := CloudFlareResponse{}
		err = json.Unmarshal(body, &cfResult)
		if err != nil {
			log.Error("Could not parse DNS response.", "err", err, "code", resp.StatusCode, "body", string(body))
			SendError(w, 500, fmt.Sprint("Could not parse DNS response. ", resp.StatusCode))
			return false
		}
		fmt.Println("cfresult:", cfResult)
		app.CloudFlareId = cfResult.Result.Id
	}

	log.Info("host registered successfully with cloudflare", "app", app)
	return true
}
