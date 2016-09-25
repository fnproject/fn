package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/iron-io/functions/api/config"
	"github.com/iron-io/functions/api/datastore"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/functions/api/mqs"
	"github.com/iron-io/functions/api/runner"
	"github.com/iron-io/functions/api/server"
	"github.com/spf13/viper"
	"golang.org/x/net/context"
)

func main() {
	ctx := context.Background()

	config.InitConfig()

	ds, err := datastore.New(viper.GetString("DB"))
	if err != nil {
		log.WithError(err).Fatalln("Invalid DB url.")
	}

	mqType, err := mqs.New(viper.GetString("MQ"))
	if err != nil {
		log.WithError(err).Fatal("Error on init MQ")
	}

	nasync := 1

	if nasyncStr := strings.TrimSpace(viper.GetString("MQADR")); len(nasyncStr) > 0 {
		var err error
		nasync, err = strconv.Atoi(nasyncStr)
		if err != nil {
			log.WithError(err).Fatalln("Failed to parse number of async runners")
		}
	}

	mqAdr := strings.TrimSpace(viper.GetString("MQADR"))
	port := viper.GetInt("PORT")
	if port == 0 {
		port = 8080
	}
	if mqAdr == "" {
		mqAdr = fmt.Sprintf("localhost:%d", port)
	}

	metricLogger := runner.NewMetricLogger()

	runner, err := runner.New(metricLogger)
	if err != nil {
		log.WithError(err).Fatalln("Failed to create a runner")
	}

	srv := server.New(ds, mqType, runner)
	go srv.Run(ctx)
	for i := 0; i < nasync; i++ {
		fmt.Println(i)
		go runAsyncRunners(mqAdr)
	}

	quit := make(chan bool)
	for _ = range quit {
	}
}

func runAsyncRunners(mqAdr string) {

	url := fmt.Sprintf("http://%s/tasks", mqAdr)

	logAndWait := func(err error) {
		log.WithError(err)
		time.Sleep(1 * time.Second)
	}

	for {
		resp, err := http.Get(url)
		if err != nil {
			logAndWait(err)
			continue
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			logAndWait(err)
			continue
		}

		var task models.Task

		if err := json.Unmarshal(body, &task); err != nil {
			logAndWait(err)
			continue
		}

		if task.ID == "" {
			time.Sleep(1 * time.Second)
			continue
		}

		log.Info("Picked up task:", task.ID)
		var stdout bytes.Buffer                                                  // TODO: should limit the size of this, error if gets too big. akin to: https://golang.org/pkg/io/#LimitReader
		stderr := runner.NewFuncLogger(task.RouteName, "", *task.Image, task.ID) // TODO: missing path here, how do i get that?

		if task.Timeout == nil {
			timeout := int32(30)
			task.Timeout = &timeout
		}
		cfg := &runner.Config{
			Image:   *task.Image,
			Timeout: time.Duration(*task.Timeout) * time.Second,
			ID:      task.ID,
			AppName: task.RouteName,
			Stdout:  &stdout,
			Stderr:  stderr,
			Env:     task.EnvVars,
		}

		metricLogger := runner.NewMetricLogger()

		rnr, err := runner.New(metricLogger)
		if err != nil {
			log.WithError(err)
			continue
		}

		ctx := context.Background()
		if _, err = rnr.Run(ctx, cfg); err != nil {
			log.WithError(err)
			continue
		}

		log.Info("Processed task:", task.ID)
		req, err := http.NewRequest(http.MethodDelete, url, bytes.NewBuffer(body))
		if err != nil {
			log.WithError(err)
		}

		c := &http.Client{}
		if _, err := c.Do(req); err != nil {
			log.WithError(err)
			continue
		}

		log.Info("Deleted task:", task.ID)
	}
}
