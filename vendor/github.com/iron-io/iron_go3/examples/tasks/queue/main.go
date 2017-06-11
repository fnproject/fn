/*
	This code sample demonstrates how to queue a worker from your your existing
	task list.

	http://dev.iron.io/worker/reference/api/
	http://dev.iron.io/worker/reference/api/#queue_a_task
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
)

type (
	TaskResponse struct {
		Message string `json:"msg"`
		Tasks   []Task `json:"tasks"`
	}

	Task struct {
		Id string `json:"id"`
	}
)

// payload defines a sample payload document
var payload = `{"tasks":[
{
  "code_name" : "Worker-Name",
  "timeout" : 20,
  "payload" : "{ \"key\" : \"value", \"key\" : \"value\" }"
}]}`

func main() {
	// Create your configuration for iron_worker
	// Find these value in credentials
	config := config.Config("iron_worker")
	config.ProjectId = "your_project_id"
	config.Token = "your_token"

	// Create your endpoint url for tasks
	url := api.ActionEndpoint(config, "tasks")
	log.Printf("Url: %s\n", url.URL.String())

	// Convert the payload to a slice of bytes
	postData := bytes.NewBufferString(payload)

	// Post the request to Iron.io
	resp, err := url.Request("POST", postData)
	defer resp.Body.Close()
	if err != nil {
		log.Println(err)
		return
	}

	// Capture the response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		return
	}

	// Unmarshall to struct
	taskResponse := &TaskResponse{}
	err = json.Unmarshal(body, taskResponse)
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
	prettyPrint(taskResponse)
}

func prettyPrint(taskResponse *TaskResponse) {
	prettyTemplate := template.Must(template.New("pretty").Parse(prettyPrintFormat()))

	tasks := "\n"
	tasks += "\"msg\": " + taskResponse.Message + "\n"
	display := new(bytes.Buffer)

	for _, task := range taskResponse.Tasks {
		display.Reset()
		prettyTemplate.Execute(display, task)
		tasks += fmt.Sprintf("%s,\n", display.String())
	}

	log.Printf(tasks)
}

func prettyPrintFormat() string {
	return `{
    "id": "{{.Id}}",
}`
}
