package testing

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/fnproject/fn/api/datastore/sql"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/models"
	"github.com/go-openapi/strfmt"
	"net/url"
	"os"
)

var testApp = &models.App{
	Name: "Test",
}

var testRoute = &models.Route{
	Path:    "/test",
	Image:   "fnproject/fn-test-utils",
	Type:    "sync",
	Format:  "http",
}

func SetupTestCall(t *testing.T, ctx context.Context, ds models.Datastore) *models.Call {
	testApp.SetDefaults()

	_, err := ds.InsertApp(ctx, testApp)
	if err != nil {
		t.Log(err.Error())
		t.Fatalf("Test InsertLog(ctx, call.ID, logText): unable to insert app, err: `%v`", err)
	}
	testRoute.AppID = testApp.ID
	_, err = ds.InsertRoute(ctx, testRoute)
	if err != nil {
		t.Log(err.Error())
		t.Fatalf("Test InsertLog(ctx, call.ID, logText): unable to insert app route, err: `%v`", err)
	}

	var call models.Call
	call.AppID = testApp.ID
	call.CreatedAt = strfmt.DateTime(time.Now())
	call.Status = "success"
	call.StartedAt = strfmt.DateTime(time.Now())
	call.CompletedAt = strfmt.DateTime(time.Now())
	call.AppName = testApp.Name
	call.Path = testRoute.Path
	return &call
}

const tmpLogDb = "/tmp/func_test_log.db"

func SetupSQLiteDS(t *testing.T) models.Datastore {
	os.Remove(tmpLogDb)
	ctx := context.Background()
	uLog, err := url.Parse("sqlite3://" + tmpLogDb)
	if err != nil {
		t.Fatalf("failed to parse url: %v", err)
	}

	ds, err := sql.New(ctx, uLog)
	if err != nil {
		t.Fatalf("failed to create sqlite3 datastore: %v", err)
	}
	return ds
}

func Test(t *testing.T, ds models.Datastore, fnl models.LogStore) {
	ctx := context.Background()
	if ds == nil {
		ds = SetupSQLiteDS(t)
	}
	call := SetupTestCall(t, ctx, ds)

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

	t.Run("call-log-not-found", func(t *testing.T) {
		call.ID = id.New().String()
		_, err := fnl.GetLog(ctx, call.AppName, call.ID)
		if err != models.ErrCallLogNotFound {
			t.Fatal("GetLog should return not found, but got:", err)
		}
	})
}
