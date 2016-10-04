package runner

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/iron-io/functions/api/models"
	"golang.org/x/net/context"
)

func getTask(url string) (*models.Task, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var task models.Task

	if err := json.Unmarshal(body, &task); err != nil {
		return nil, err
	}

	if task.ID == "" {
		return nil, errors.New("Invalid Task: ID empty")
	}
	return &task, nil
}

func getCfg(task *models.Task) *Config {
	var stdout bytes.Buffer                                           // TODO: should limit the size of this, error if gets too big. akin to: https://golang.org/pkg/io/#LimitReader
	stderr := NewFuncLogger(task.RouteName, "", *task.Image, task.ID) // TODO: missing path here, how do i get that?
	if task.Timeout == nil {
		timeout := int32(30)
		task.Timeout = &timeout
	}
	cfg := &Config{
		Image:   *task.Image,
		Timeout: time.Duration(*task.Timeout) * time.Second,
		ID:      task.ID,
		AppName: task.RouteName,
		Stdout:  &stdout,
		Stderr:  stderr,
		Env:     task.EnvVars,
	}
	return cfg
}

func deleteTask(url string, task *models.Task) error {
	// Unmarshal task to be sent over as a json
	body, err := json.Marshal(task)
	if err != nil {
		return err
	}

	// Send out Delete request to delete task from queue
	req, err := http.NewRequest(http.MethodDelete, url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	c := &http.Client{}
	if resp, err := c.Do(req); err != nil {
		return err
	} else if resp.StatusCode != http.StatusAccepted {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return errors.New(string(body))
	}
	return nil
}

func runTask(task *models.Task) error {
	// Set up runner and process task
	cfg := getCfg(task)
	ctx := context.Background()
	rnr, err := New(NewMetricLogger())
	if err != nil {
		return err
	}
	_, err = rnr.Run(ctx, cfg)
	return err
}

// RunAsyncRunner pulls tasks off a queue and processes them
func RunAsyncRunner(tasksrv, port string) {
	u := tasksrvURL(tasksrv, port)
	for {
		task, err := getTask(u)
		if err != nil {
			log.WithError(err)
			time.Sleep(1 * time.Second)
			continue
		}
		log.Info("Picked up task:", task.ID)

		// Process Task
		if err := runTask(task); err != nil {
			log.WithError(err)
			continue
		}
		log.Info("Processed task:", task.ID)

		// Delete task from queue
		if err := deleteTask(u, task); err != nil {
			log.WithError(err)
		} else {
			log.Info("Deleted task:", task.ID)
		}
	}
}

func tasksrvURL(tasksrv, port string) string {
	parsed, err := url.Parse(tasksrv)
	if err != nil {
		log.Fatalf("cannot parse TASKSRV endpoint: %v", err)
	}

	if parsed.Scheme == "" {
		parsed.Scheme = "http"
	}

	if parsed.Path == "" || parsed.Path == "/" {
		parsed.Path = "/tasks"
	}

	if _, _, err := net.SplitHostPort(parsed.Host); err != nil {
		parsed.Host = net.JoinHostPort(parsed.Host, port)
	}

	return parsed.String()
}
