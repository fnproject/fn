package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/functions/api/mqs"
)

func getMockTask() models.Task {
	priority := int32(0)
	image := fmt.Sprintf("Image-%d", rand.Int31()%1000)
	task := &models.Task{}
	task.Image = &image
	task.ID = fmt.Sprintf("ID-%d", rand.Int31()%1000)
	task.RouteName = fmt.Sprintf("RouteName-%d", rand.Int31()%1000)
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
		c.JSON(http.StatusAccepted, task)
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
	mockTask := getMockTask()

	ts := getTestServer([]*models.Task{&mockTask})
	defer ts.Close()

	url := ts.URL + "/tasks"
	task, err := getTask(url)
	if err != nil {
		t.Error("expected no error, got", err)
	}
	if task.ID != mockTask.ID {
		t.Errorf("expected task ID '%s', got '%s'", task.ID, mockTask.ID)
	}
}

func TestGetTaskError(t *testing.T) {
	tests := []map[string]interface{}{
		map[string]interface{}{
			"url":   "/invalid",
			"task":  getMockTask(),
			"error": "invalid character 'p' after top-level value",
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
		_, err := getTask(url)
		if err == nil {
			t.Errorf("expected error '%s'", test["error"].(string))
		}
		if err.Error() != test["error"].(string) {
			t.Errorf("test %d: expected error '%s', got '%s'", i, test["error"].(string), err)
		}
	}
}

func TestDeleteTask(t *testing.T) {
	mockTask := getMockTask()

	ts := getTestServer([]*models.Task{&mockTask})
	defer ts.Close()

	url := ts.URL + "/tasks"
	err := deleteTask(url, &mockTask)
	if err == nil {
		t.Error("expected error 'Not reserver', got", err)
	}

	_, err = getTask(url)
	if err != nil {
		t.Error("expected no error, got", err)
	}

	err = deleteTask(url, &mockTask)
	if err != nil {
		t.Error("expected no error, got", err)
	}
}
