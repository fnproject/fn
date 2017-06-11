/*
	This code sample demonstrates how to get the log for a task

	http://dev.iron.io/worker/reference/api/
	http://dev.iron.io/worker/reference/api/#get_a_tasks_log
*/
package main

import (
	"github.com/iron-io/iron_go3/api"
	"github.com/iron-io/iron_go3/config"
	"io/ioutil"
	"log"
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
	url := api.Action(config, "tasks", taskId, "log")
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

	// Display the log
	log.Printf("\n%s\n", string(body))
}
