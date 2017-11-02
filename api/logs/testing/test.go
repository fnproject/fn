package testing

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/models"
	"github.com/go-openapi/strfmt"
)

var testApp = &models.App{
	Name: "Test",
}

var testRoute = &models.Route{
	AppName: testApp.Name,
	Path:    "/test",
	Image:   "fnproject/hello",
	Type:    "sync",
	Format:  "http",
}

func SetupTestCall() *models.Call {
	var call models.Call
	call.CreatedAt = strfmt.DateTime(time.Now())
	call.Status = "success"
	call.StartedAt = strfmt.DateTime(time.Now())
	call.CompletedAt = strfmt.DateTime(time.Now())
	call.AppName = testApp.Name
	call.Path = testRoute.Path
	return &call
}

func Test(t *testing.T, fnl models.LogStore) {
	ctx := context.Background()
	call := SetupTestCall()

	t.Run("call-log-insert-get", func(t *testing.T) {
		call.ID = id.New().String()
		logText := "test"
		log := strings.NewReader(logText)
		err := fnl.InsertLog(ctx, call.AppName, call.ID, log)
		if err != nil {
			t.Fatalf("Test InsertLog(ctx, call.ID, logText): unexpected error during inserting log `%v`", err)
		}
		logEntry, err := fnl.GetLog(ctx, call.AppName, call.ID)
		var b bytes.Buffer
		io.Copy(&b, logEntry)
		if !strings.Contains(b.String(), logText) {
			t.Fatalf("Test GetLog(ctx, call.ID, logText): unexpected error, log mismatch. "+
				"Expected: `%v`. Got `%v`.", logText, b.String())
		}
	})
}
