package runner

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/runner/common"
	taskpkg "github.com/fnproject/fn/api/runner/task"
	"github.com/opentracing/opentracing-go"
)

var client = &http.Client{
	Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 120 * time.Second,
		}).Dial,
		MaxIdleConnsPerHost: 512,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig: &tls.Config{
			ClientSessionCache: tls.NewLRUClientSessionCache(4096),
		},
	},
}

func getTask(ctx context.Context, url string) (*models.Task, error) {
	// TODO shove this ctx into the request?
	span, _ := opentracing.StartSpanFromContext(ctx, "get_task")
	defer span.Finish()

	req, _ := http.NewRequest(http.MethodGet, url, nil)
	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer func() {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("Unable to get task. Reason: %v", resp.Status))
	}

	var task models.Task
	err = json.NewDecoder(resp.Body).Decode(&task)
	if err != nil {
		return nil, err
	}
	if task.ID == "" {
		return nil, nil
	}
	return &task, nil
}

func deleteTask(ctx context.Context, url string, task *models.Task) error {
	span, _ := opentracing.StartSpanFromContext(ctx, "delete_task")
	defer span.Finish()

	// Unmarshal task to be sent over as a json
	body, err := json.Marshal(task)
	if err != nil {
		return err
	}

	// TODO use a reasonable http client..
	// Send out Delete request to delete task from queue
	req, err := http.NewRequest(http.MethodDelete, url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusAccepted {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return errors.New(string(body))
	}
	return nil
}

// RunAsyncRunner pulls tasks off a queue and processes them
func RunAsyncRunner(ctx context.Context, tasksrv string, rnr *Runner, ds models.Datastore) {
	u := tasksrvURL(tasksrv)

	startAsyncRunners(ctx, u, rnr, ds)
	<-ctx.Done()
}

func startAsyncRunners(ctx context.Context, url string, rnr *Runner, ds models.Datastore) {
	var wg sync.WaitGroup
	ctx, log := common.LoggerWithFields(ctx, logrus.Fields{"runner": "async"})
	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return
		default:
		}

		if !rnr.hasAsyncAvailableMemory() { // TODO this should be a channel to subscribe to
			log.Debug("memory full")
			time.Sleep(1 * time.Second)
			continue
		}

		runAsyncTask(ctx, url, rnr, ds, &wg)
	}
}

func runAsyncTask(ctx context.Context, url string, rnr *Runner, ds models.Datastore, wg *sync.WaitGroup) {
	// start a new span altogether, unrelated to the shared global context
	span := opentracing.GlobalTracer().StartSpan("async_task")
	ctx = opentracing.ContextWithSpan(ctx, span)
	defer span.Finish()
	log := common.Logger(ctx)

	task, err := getTask(ctx, url)
	if err != nil {
		if err, ok := err.(net.Error); ok && err.Timeout() {
			log.WithError(err).Errorln("Could not fetch task, timeout.")
			return
		}
		log.WithError(err).Error("Could not fetch task")
		time.Sleep(1 * time.Second)
		return
	}
	if task == nil {
		time.Sleep(1 * time.Second)
		return
	}

	ctx, log = common.LoggerWithFields(ctx, logrus.Fields{"call_id": task.ID})
	log.Info("Running task async:", task.ID)

	wg.Add(1)

	go func() {
		defer wg.Done()
		// Process Task
		_, err := rnr.RunTrackedTask(task, ctx, taskpkg.ConfigFromTask(task))
		if err != nil {
			log.WithError(err).Error("Cannot run task")
		}
		log.Debug("Processed task")
	}()

	// TODO this is so wrong... fix later+asap

	// Delete task from queue
	if err := deleteTask(ctx, url, task); err != nil {
		log.WithError(err).Error("Cannot delete task")
		return
	}

	// TODO uh, even if we don't delete it it still runs but w/e
	log.Info("Task complete")
}

func tasksrvURL(tasksrv string) string {
	parsed, err := url.Parse(tasksrv)
	if err != nil {
		logrus.WithError(err).Fatalln("cannot parse API_URL endpoint")
	}

	if parsed.Scheme == "" {
		parsed.Scheme = "http"
	}

	if parsed.Path == "" || parsed.Path == "/" {
		parsed.Path = "/tasks"
	}

	return parsed.String()
}
