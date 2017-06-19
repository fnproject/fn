package testing

import (
	"testing"
	"time"
	"context"
	"strings"

	"gitlab-odx.oracle.com/odx/functions/api/models"
	"github.com/go-openapi/strfmt"
	"gitlab-odx.oracle.com/odx/functions/api/id"
)


var testApp = &models.App{
	Name: "Test",
}

var testRoute = &models.Route{
	AppName: testApp.Name,
	Path:    "/test",
	Image:   "funcy/hello",
	Type:    "sync",
	Format:  "http",
}

func SetUpTestTask() *models.Task {
	task := &models.Task{}
	task.CreatedAt = strfmt.DateTime(time.Now())
	task.Status = "success"
	task.StartedAt = strfmt.DateTime(time.Now())
	task.CompletedAt = strfmt.DateTime(time.Now())
	task.AppName = testApp.Name
	task.Path = testRoute.Path
	return task
}

func Test(t *testing.T, fnl models.FnLog, ds models.Datastore) {
	ctx := context.Background()
	task := SetUpTestTask()

	t.Run("call-log-insert", func(t *testing.T) {
		task.ID = id.New().String()
		err := ds.InsertTask(ctx, task)
		if err != nil {
			t.Fatalf("Test InsertTask(ctx, &task): unexpected error `%v`", err)
		}
		err = fnl.InsertLog(ctx, task.ID, "test")
		if err != nil {
			t.Fatalf("Test InsertLog(ctx, task.ID, logText): unexpected error during inserting log `%v`", err)
		}
	})
	t.Run("call-log-insert-get", func(t *testing.T) {
		task.ID = id.New().String()
		err := ds.InsertTask(ctx, task)
		logText := "test"
		if err != nil {
			t.Fatalf("Test InsertTask(ctx, &task): unexpected error `%v`", err)
		}
		err = fnl.InsertLog(ctx, task.ID, logText)
		if err != nil {
			t.Fatalf("Test InsertLog(ctx, task.ID, logText): unexpected error during inserting log `%v`", err)
		}
		logEntry, err := fnl.GetLog(ctx, task.ID)
		if !strings.Contains(logEntry.Log, logText) {
			t.Fatalf("Test GetLog(ctx, task.ID, logText): unexpected error, log mismatch. " +
				"Expected: `%v`. Got `%v`.", logText, logEntry.Log)
		}
	})
	t.Run("call-log-insert-get-delete", func(t *testing.T) {
		task.ID = id.New().String()
		err := ds.InsertTask(ctx, task)
		logText := "test"
		if err != nil {
			t.Fatalf("Test InsertTask(ctx, &task): unexpected error `%v`", err)
		}
		err = fnl.InsertLog(ctx, task.ID, logText)
		if err != nil {
			t.Fatalf("Test InsertLog(ctx, task.ID, logText): unexpected error during inserting log `%v`", err)
		}
		logEntry, err := fnl.GetLog(ctx, task.ID)
		if !strings.Contains(logEntry.Log, logText) {
			t.Fatalf("Test GetLog(ctx, task.ID, logText): unexpected error, log mismatch. " +
				"Expected: `%v`. Got `%v`.", logText, logEntry.Log)
		}
		err = fnl.DeleteLog(ctx, task.ID)
		if err != nil {
			t.Fatalf("Test DeleteLog(ctx, task.ID): unexpected error during deleting log `%v`", err)
		}
	})
}
