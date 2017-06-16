/*
	This code sample demonstrates how to get a list of existing tasks

	http://dev.iron.io/worker/reference/api/
	http://dev.iron.io/worker/reference/api/#list_tasks
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
	TaskResponse struct {
		Tasks []Task `json:"tasks"`
	}

	Task struct {
		Id        string    `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		ProjectId string    `json:"project_id"`
		CodeId    string    `json:"code_id"`
		Status    string    `json:"status"`
		Message   string    `json:"msg"`
		CodeName  string    `json:"code_name"`
		StartTime time.Time `json:"start_time"`
		EndTime   time.Time `json:"end_time"`
		Duration  int       `json:"duration"`
		RunTimes  int       `json:"run_times"`
		Timeout   int       `json:"timeout"`
		Percent   int       `json:"percent"`
	}
)

func main() {
	// Create your configuration for iron_worker
	// Find these value in credentials
	config := config.Config("iron_worker")
	config.ProjectId = "your_project_id"
	config.Token = "your_token"

	// Create your endpoint url for tasks
	url := api.ActionEndpoint(config, "tasks")
	url.QueryAdd("code_name", "%s", "task")
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
    "created_at": "{{.CreatedAt}}",
    "updated_at": "{{.UpdatedAt}}",
    "project_id": "{{.ProjectId}}",
    "code_id": "{{.CodeId}}",
    "status": "{{.Status}}",
    "msg": "{{.Message}}",
    "code_name": "{{.CodeName}}",
    "start_time": "{{.StartTime}}",
    "end_time": "{{.EndTime}}",
    "duration": {{.Duration}},
    "run_times": {{.RunTimes}},
    "timeout": {{.Timeout}},
    "percent": {{.Percent}}
}`
}
