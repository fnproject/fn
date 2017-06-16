/*
	This code sample demonstrates how to get a list of existing tasks

	http://dev.iron.io/worker/reference/api/
	http://dev.iron.io/worker/reference/api/#get_info_about_a_code_package
*/
package main

import (
	"bytes"
	"encoding/json"
	"github.com/iron-io/iron_go3/api"
	"github.com/iron-io/iron_go3/config"
	"io/ioutil"
	"log"
	"text/template"
	"time"
)

type (
	Code struct {
		Id              string    `json:"id"`
		ProjectId       string    `json:"project_id"`
		Name            string    `json:"name"`
		Runtime         string    `json:"runtime"`
		LatestChecksum  string    `json:"latest_checksum"`
		Revision        int       `json:"rev"`
		LatestHistoryId string    `json:"latest_history_id"`
		LatestChange    time.Time `json:"latest_change"`
	}
)

func main() {
	// Create your configuration for iron_worker
	// Find these value in credentials
	config := config.Config("iron_worker")
	config.ProjectId = "your_project_id"
	config.Token = "your_token"

	// Capture info for this code
	codeId := "522d160a91c530531f6f528d"

	// Create your endpoint url for tasks
	url := api.Action(config, "codes", codeId)
	log.Printf("Url: %s\n", url.URL.String())

	// Post the request to Iron.io
	resp, err := url.Request("GET", nil)
	defer resp.Body.Close()
	if err != nil {
		log.Println(err)
		return
	}

	// Check the status code
	if resp.StatusCode != 200 {
		log.Printf("%v\n", resp)
		return
	}

	// Capture the response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		return
	}

	// Unmarshall to struct
	code := &Code{}
	err = json.Unmarshal(body, code)
	if err != nil {
		log.Printf("%v\n", err)
		return
	}

	// Unmarshall to map
	results := map[string]interface{}{}
	err = json.Unmarshal(body, &results)
	if err != nil {
		log.Printf("%v\n", err)
		return
	}

	// Pretty print the response
	prettyPrint(code)
}

func prettyPrint(code *Code) {
	prettyTemplate := template.Must(template.New("pretty").Parse(prettyPrintFormat()))

	display := new(bytes.Buffer)

	prettyTemplate.Execute(display, code)
	log.Printf("%s,\n", display.String())
}

func prettyPrintFormat() string {
	return `{
    "id": "{{.Id}}",
    "project_id": "{{.ProjectId}}",
    "name": "{{.Name}}",
    "runtime": "{{.Runtime}}",
    "latest_checksum": "{{.LatestChecksum}}",
    "rev": {{.Revision}},
    "latest_history_id": "{{.LatestHistoryId}}",
    "latest_change": "{{.LatestChange}}",
}`
}
