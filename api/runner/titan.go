package runner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/iron-io/functions/api/models"
	tmodels "github.com/iron-io/titan/jobserver/models"
)

type TitanJob struct {
	runner     *RouteRunner
	resultChan chan error
	result     []byte
}

var versionPath = "/v1"

func CreateTitanJob(runner *RouteRunner) *TitanJob {
	t := &TitanJob{
		runner:     runner,
		resultChan: make(chan error),
	}

	go t.Start()

	return t
}

func (t *TitanJob) Start() {
	newjob := tmodels.JobsWrapper{
		Jobs: []*tmodels.Job{
			&tmodels.Job{
				NewJob: tmodels.NewJob{
					Image:   &t.runner.Route.Image,
					Payload: t.runner.Payload,
				},
			},
		},
	}

	jobJSON, err := json.Marshal(newjob)
	if err != nil {
		t.resultChan <- models.ErrInvalidJSON
		return
	}

	resp, err := t.titanPOST(fmt.Sprintf("/groups/app-%s/jobs", t.runner.Route.AppName), bytes.NewBuffer(jobJSON))
	if err != nil {
		t.resultChan <- models.ErrRunnerAPICantConnect
		return
	}

	var resultJobs tmodels.JobsWrapper
	respBody, err := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(respBody, &resultJobs)
	if err != nil {
		t.resultChan <- models.ErrInvalidJSON
		return
	}

	if resultJobs.Jobs == nil {
		t.resultChan <- models.ErrRunnerAPICreateJob
		return
	}

	job := resultJobs.Jobs[0]
	begin := time.Now()
	for len(t.result) == 0 {
		if time.Since(begin) > t.runner.Timeout {
			t.resultChan <- models.ErrRunnerTimeout
			return
		}

		resp, err := t.titanGET(fmt.Sprintf("/groups/app-%s/jobs/%s/log", t.runner.Route.AppName, job.ID))
		if err == nil {
			fmt.Println(resp.Status)
			if resp.StatusCode == http.StatusOK {
				resBody, err := ioutil.ReadAll(resp.Body)
				fmt.Println(string(resBody))
				if err != nil {
					t.resultChan <- models.ErrRunnerInvalidResponse
					return
				}

				t.result = resBody
				continue
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.resultChan <- nil
}

func (t *TitanJob) Wait() error {
	return <-t.resultChan
}

func (t TitanJob) Result() []byte {
	return t.result
}

func (t TitanJob) titanPOST(path string, body io.Reader) (*http.Response, error) {
	fmt.Println(fmt.Sprintf("%s%s%s", t.runner.Endpoint, versionPath, path))
	return http.Post(fmt.Sprintf("%s%s%s", t.runner.Endpoint, versionPath, path), "application/json", body)
}

func (t TitanJob) titanGET(path string) (*http.Response, error) {
	fmt.Println(fmt.Sprintf("%s%s%s", t.runner.Endpoint, versionPath, path))
	return http.Get(fmt.Sprintf("%s%s%s", t.runner.Endpoint, versionPath, path))
}
