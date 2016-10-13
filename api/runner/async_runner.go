package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/runner/common"
	"github.com/iron-io/runner/drivers"
)

func getTask(ctx context.Context, url string) (*models.Task, error) {
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
		return nil, nil
	}
	return &task, nil
}

func getCfg(task *models.Task) *Config {
	// TODO: should limit the size of this, error if gets too big. akin to: https://golang.org/pkg/io/#LimitReader
	stderr := NewFuncLogger(task.AppName, task.Path, *task.Image, task.ID) // TODO: missing path here, how do i get that?
	if task.Timeout == nil {
		timeout := int32(30)
		task.Timeout = &timeout
	}
	cfg := &Config{
		Image:   *task.Image,
		Timeout: time.Duration(*task.Timeout) * time.Second,
		ID:      task.ID,
		AppName: task.AppName,
		Stdout:  stderr,
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

func runTask(ctx context.Context, task *models.Task) (drivers.RunResult, error) {
	// Set up runner and process task
	cfg := getCfg(task)
	rnr, err := New(NewMetricLogger())
	if err != nil {
		return nil, err
	}
	return rnr.Run(ctx, cfg)
}

// RunAsyncRunner pulls tasks off a queue and processes them
func RunAsyncRunner(ctx context.Context, tasksrv string, n int) {
	u, h := tasksrvURL(tasksrv)
	if isHostOpen(h) {
		return
	}

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go startAsyncRunners(ctx, &wg, i, u, runTask)
	}

	wg.Wait()
	<-ctx.Done()
}

func isHostOpen(host string) bool {
	conn, err := net.Dial("tcp", host)
	available := err == nil
	if available {
		conn.Close()
	}
	return available
}

// todo: not a big fan of this anonymous function for testing, should use an interface and make a Mock object for testing - TR
func startAsyncRunners(ctx context.Context, wg *sync.WaitGroup, i int, url string, runTask func(ctx context.Context, task *models.Task) (drivers.RunResult, error)) {
	ctx, log := common.LoggerWithFields(ctx, logrus.Fields{"async_runner": i})
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return

		default:
			task, err := getTask(ctx, url)
			if err != nil {
				if err, ok := err.(net.Error); ok && err.Timeout() {
					log.WithError(err).Errorln("Could not fetch task, timeout.")
					continue
				}
				log.WithError(err).Error("Could not fetch task")
				time.Sleep(1 * time.Second)
				continue
			}
			if task == nil {
				time.Sleep(1 * time.Second)
				continue
			}

			ctx, log := common.LoggerWithFields(ctx, logrus.Fields{"call_id": task.ID})
			log.Debug("Running task:", task.ID)
			// Process Task
			if _, err := runTask(ctx, task); err != nil {
				log.WithError(err).Error("Cannot run task")
				continue
			}
			log.Debug("Processed task")

			// Delete task from queue
			if err := deleteTask(url, task); err != nil {
				log.WithError(err).Error("Cannot delete task")
				continue
			}

			log.Info("Task complete")
		}
	}
}

func tasksrvURL(tasksrv string) (parsedURL, host string) {
	parsed, err := url.Parse(tasksrv)
	if err != nil {
		logrus.WithError(err).Fatalln("cannot parse TASKSRV endpoint")
	}
	// host, port, err := net.SplitHostPort(parsed.Host)
	// if err != nil {
	// 	log.WithError(err).Fatalln("net.SplitHostPort")
	// }

	if parsed.Scheme == "" {
		parsed.Scheme = "http"
	}

	if parsed.Path == "" || parsed.Path == "/" {
		parsed.Path = "/tasks"
	}

	// if _, _, err := net.SplitHostPort(parsed.Host); err != nil {
	// 	parsed.Host = net.JoinHostPort(parsed.Host, parsed)
	// }

	return parsed.String(), parsed.Host
}
