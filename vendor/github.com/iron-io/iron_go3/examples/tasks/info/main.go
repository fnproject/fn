/*
	This code sample demonstrates how to get a list of existing tasks

	http://dev.iron.io/worker/reference/api/
	http://dev.iron.io/worker/reference/api/#get_info_about_a_task
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
	Task struct {
		Id            string    `json:"id"`
		ProjectId     string    `json:"project_id"`
		CodeId        string    `json:"code_id"`
		CodeHistoryId string    `json:"code_history_id"`
		Status        string    `json:"status"`
		CodeName      string    `json:"code_name"`
		CodeRevision  string    `json:"code_rev"`
		StartTime     time.Time `json:"start_time"`
		EndTime       time.Time `json:"end_time"`
		Duration      int       `json:"duration"`
		Timeout       int       `json:"timeout"`
		Payload       string    `json:"payload"`
		UpdatedAt     time.Time `json:"updated_at"`
		CreatedAt     time.Time `json:"created_at"`
	}
)

func main() {
	// Create your configuration for iron_worker
	// Find these value in credentials
	config := config.Config("iron_worker")
	config.ProjectId = "your_project_id"
	config.Token = "your_token"

	// Capture info for this task
	taskId := "52b45b17a31186632b00da4c"

	// Create your endpoint url for tasks
	url := api.Action(config, "tasks", taskId)
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
	task := &Task{}
	err = json.Unmarshal(body, task)
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
	prettyPrint(task)
}

func prettyPrint(task *Task) {
	prettyTemplate := template.Must(template.New("pretty").Parse(prettyPrintFormat()))

	display := new(bytes.Buffer)

	prettyTemplate.Execute(display, task)
	log.Printf("%s,\n", display.String())
}

func prettyPrintFormat() string {
	return `{
    "id": "{{.Id}}",
    "project_id": "{{.ProjectId}}",
    "code_id": "{{.CodeId}}",
    "code_history_id": "{{.CodeHistoryId}}",
    "status": "{{.Status}}",
    "code_name": "{{.CodeName}}",
    "code_revision": "{{.CodeRevision}}",
    "start_time": "{{.StartTime}}",
    "end_time": "{{.EndTime}}",
    "duration": {{.Duration}},
    "timeout": {{.Timeout}},
    "payload": {{.Payload}},
    "created_at": "{{.CreatedAt}}",
    "updated_at": "{{.UpdatedAt}}",
}`
}
