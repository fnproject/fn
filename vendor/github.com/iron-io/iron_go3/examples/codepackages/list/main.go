/*
	This code sample demonstrates how to get a list of existing tasks

	http://dev.iron.io/worker/reference/api/
	http://dev.iron.io/worker/reference/api/#list_code_packages
*/
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/iron-io/iron_go3/api"
	"github.com/iron-io/iron_go3/config"
	"io/ioutil"
	"log"
	"text/template"
	"time"
)

type (
	CodeResponse struct {
		Codes []Code `json:"codes"`
	}

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

	// Create your endpoint url for tasks
	url := api.ActionEndpoint(config, "codes")
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
	codeResponse := &CodeResponse{}
	err = json.Unmarshal(body, codeResponse)
	if err != nil {
		log.Printf("%v\n", err)
		return
	}

	// Or you can Unmarshall to map
	results := map[string]interface{}{}
	err = json.Unmarshal(body, &results)
	if err != nil {
		log.Printf("%v\n", err)
		return
	}

	// Pretty print the response
	prettyPrint(codeResponse)
}

func prettyPrint(codeResponse *CodeResponse) {
	prettyTemplate := template.Must(template.New("pretty").Parse(prettyPrintFormat()))

	codes := "\n"
	display := new(bytes.Buffer)

	for _, code := range codeResponse.Codes {
		display.Reset()
		prettyTemplate.Execute(display, code)
		codes += fmt.Sprintf("%s,\n", display.String())
	}

	log.Printf(codes)
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
