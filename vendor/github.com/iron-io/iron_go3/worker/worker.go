// IronWorker (elastic computing) client library
package worker

import (
	"time"

	"github.com/iron-io/iron_go3/api"
	"github.com/iron-io/iron_go3/config"
)

type Worker struct {
	Settings config.Settings
}

func New() *Worker {
	return &Worker{Settings: config.Config("iron_worker")}
}

func (w *Worker) codes(s ...string) *api.URL     { return api.Action(w.Settings, "codes", s...) }
func (w *Worker) tasks(s ...string) *api.URL     { return api.Action(w.Settings, "tasks", s...) }
func (w *Worker) schedules(s ...string) *api.URL { return api.Action(w.Settings, "schedules", s...) }
func (w *Worker) clusters(s ...string) *api.URL  { return api.RootAction(w.Settings, "clusters", s...) }

// exponential sleep between retries, replace this with your own preferred strategy
func sleepBetweenRetries(previousDuration time.Duration) time.Duration {
	if previousDuration >= 60*time.Second {
		return previousDuration
	}
	return previousDuration + previousDuration
}

var GoCodeRunner = []byte(`#!/bin/sh
root() {
  while [ $# -gt 0 ]; do
    if [ "$1" = "-d" ]; then
      printf "%s\n" "$2"
      break
    fi
  done
}
cd "$(root "$@")"
chmod +x worker
./worker "$@"
`)

// WaitForTask returns a channel that will receive the completed task and is closed afterwards.
// If an error occured during the wait, the channel will be closed.
func (w *Worker) WaitForTask(taskId string) chan TaskInfo {
	out := make(chan TaskInfo)
	go func() {
		defer close(out)
		retryDelay := 100 * time.Millisecond

		for {
			info, err := w.TaskInfo(taskId)
			if err != nil {
				return
			}

			if info.Status == "queued" || info.Status == "running" {
				time.Sleep(retryDelay)
				retryDelay = sleepBetweenRetries(retryDelay)
			} else {
				out <- info
				return
			}
		}
	}()

	return out
}

func (w *Worker) WaitForTaskLog(taskId string) chan []byte {
	out := make(chan []byte)

	go func() {
		defer close(out)
		retryDelay := 100 * time.Millisecond

		for {
			log, err := w.TaskLog(taskId)
			if err != nil {
				e, ok := err.(api.HTTPResponseError)
				if ok && e.StatusCode() == 404 {
					time.Sleep(retryDelay)
					retryDelay = sleepBetweenRetries(retryDelay)
					continue
				}
				return
			}
			out <- log
			return
		}
	}()
	return out
}

func clamp(value, min, max int) int {
	if value < min {
		return min
	} else if value > max {
		return max
	}
	return value
}
