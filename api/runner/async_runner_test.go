package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/fnproject/fn/api/datastore"
	"github.com/fnproject/fn/api/logs"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/mqs"
	"github.com/fnproject/fn/api/runner/drivers"
	"github.com/fnproject/fn/api/runner/task"
	"github.com/gin-gonic/gin"
)

func setLogBuffer() *bytes.Buffer {
	var buf bytes.Buffer
	buf.WriteByte('\n')
	logrus.SetOutput(&buf)
	gin.DefaultErrorWriter = &buf
	gin.DefaultWriter = &buf
	log.SetOutput(&buf)
	return &buf
}

func getMockTask() models.Task {
	priority := int32(0)
	image := fmt.Sprintf("Image-%d", rand.Int31()%1000)
	task := &models.Task{}
	task.Image = image
	task.ID = fmt.Sprintf("ID-%d", rand.Int31()%1000)
	task.AppName = fmt.Sprintf("RouteName-%d", rand.Int31()%1000)
	task.Priority = &priority
	return *task
}

func getTestServer(mockTasks []*models.Task) *httptest.Server {
	ctx := context.TODO()

	mq, err := mqs.New("memory://test")
	if err != nil {
		panic(err)
	}

	for _, mt := range mockTasks {
		mq.Push(ctx, mt)
	}

	getHandler := func(c *gin.Context) {
		task, err := mq.Reserve(ctx)
		if err != nil {
			logrus.WithError(err)
			c.JSON(http.StatusInternalServerError, err)
			return
		}
		c.JSON(http.StatusOK, task)
	}

	delHandler := func(c *gin.Context) {
		body, err := ioutil.ReadAll(c.Request.Body)
		if err != nil {
			logrus.WithError(err)
			c.JSON(http.StatusInternalServerError, err.Error())
			return
		}
		var task models.Task
		if err = json.Unmarshal(body, &task); err != nil {
			logrus.WithError(err)
			c.JSON(http.StatusInternalServerError, err.Error())
			return
		}

		if err := mq.Delete(ctx, &task); err != nil {
			logrus.WithError(err)
			c.JSON(http.StatusInternalServerError, err.Error())
			return
		}
		c.JSON(http.StatusAccepted, task)
	}

	r := gin.Default()
	r.GET("/tasks", getHandler)
	r.DELETE("/tasks", delHandler)
	return httptest.NewServer(r)
}

func TestGetTask(t *testing.T) {
	buf := setLogBuffer()
	mockTask := getMockTask()

	ts := getTestServer([]*models.Task{&mockTask})
	defer ts.Close()

	url := ts.URL + "/tasks"
	task, err := getTask(context.Background(), url)
	if err != nil {
		t.Log(buf.String())
		t.Error("expected no error, got", err)
	}
	if task.ID != mockTask.ID {
		t.Log(buf.String())
		t.Errorf("expected task ID '%s', got '%s'", task.ID, mockTask.ID)
	}
}

func TestGetTaskError(t *testing.T) {
	buf := setLogBuffer()

	tests := []map[string]interface{}{
		{
			"url":   "/invalid",
			"task":  getMockTask(),
			"error": "Unable to get task. Reason: 404 Not Found",
		},
	}

	var tasks []*models.Task
	for _, v := range tests {
		task := v["task"].(models.Task)
		tasks = append(tasks, &task)
	}

	ts := getTestServer(tasks)
	defer ts.Close()

	for i, test := range tests {
		url := ts.URL + test["url"].(string)
		_, err := getTask(context.Background(), url)
		if err == nil {
			t.Log(buf.String())
			t.Errorf("expected error '%s'", test["error"].(string))
		}
		if err.Error() != test["error"].(string) {
			t.Log(buf.String())
			t.Errorf("test %d: expected error '%s', got '%s'", i, test["error"].(string), err)
		}
	}
}

func TestDeleteTask(t *testing.T) {
	buf := setLogBuffer()
	mockTask := getMockTask()
	ctx := context.Background()

	ts := getTestServer([]*models.Task{&mockTask})
	defer ts.Close()

	url := ts.URL + "/tasks"
	err := deleteTask(ctx, url, &mockTask)
	if err == nil {
		t.Log(buf.String())
		t.Error("expected error 'Not reserver', got", err)
	}

	_, err = getTask(ctx, url)
	if err != nil {
		t.Log(buf.String())
		t.Error("expected no error, got", err)
	}

	err = deleteTask(ctx, url, &mockTask)
	if err != nil {
		t.Log(buf.String())
		t.Error("expected no error, got", err)
	}
}

func TestTasksrvURL(t *testing.T) {
	tests := []struct {
		in, out string
	}{
		{"//localhost:8081", "http://localhost:8081/tasks"},
		{"//localhost:8081/", "http://localhost:8081/tasks"},
		{"http://localhost:8081", "http://localhost:8081/tasks"},
		{"http://localhost:8081/", "http://localhost:8081/tasks"},
		{"http://localhost:8081/endpoint", "http://localhost:8081/endpoint"},
	}

	for _, tt := range tests {
		if got := tasksrvURL(tt.in); got != tt.out {
			t.Errorf("tasksrv: %s\texpected: %s\tgot: %s\t", tt.in, tt.out, got)
		}
	}
}

func testRunner(t *testing.T) (*Runner, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	ds := datastore.NewMock()
	fnl := logs.NewMock()
	r, err := New(ctx, NewFuncLogger(fnl), ds)
	if err != nil {
		t.Fatal("Test: failed to create new runner")
	}
	return r, cancel
}

type RunResult struct {
	drivers.RunResult
}

func (r RunResult) Status() string {
	return "success"
}

func TestAsyncRunnersGracefulShutdown(t *testing.T) {
	buf := setLogBuffer()
	mockTask := getMockTask()
	ts := getTestServer([]*models.Task{&mockTask})
	defer ts.Close()

	tasks := make(chan task.Request)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	defer close(tasks)
	go func() {
		for t := range tasks {
			t.Response <- task.Response{
				Result: RunResult{},
				Err:    nil,
			}

		}
	}()
	rnr, cancel := testRunner(t)
	defer cancel()
	startAsyncRunners(ctx, ts.URL+"/tasks", rnr, datastore.NewMock())

	if err := ctx.Err(); err != context.DeadlineExceeded {
		t.Log(buf.String())
		t.Errorf("async runners stopped unexpectedly. context error: %v", err)
	}
}
