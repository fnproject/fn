package testing

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/models"
)

var testApp = &models.App{
	Name: "Test",
	ID:   id.New().String(),
}

var testRoute = &models.Route{
	Path:   "/test",
	Image:  "fnproject/fn-test-utils",
	Type:   "sync",
	Format: "http",
}

func SetupTestCall(t *testing.T, ctx context.Context, ls models.LogStore) *models.Call {
	var call models.Call
	call.AppID = testApp.ID
	call.CreatedAt = common.DateTime(time.Now())
	call.Status = "success"
	call.StartedAt = common.DateTime(time.Now())
	call.CompletedAt = common.DateTime(time.Now())
	call.Path = testRoute.Path
	return &call
}

const tmpLogDb = "/tmp/func_test_log.db"

func Test(t *testing.T, fnl models.LogStore) {
	ctx := context.Background()
	call := SetupTestCall(t, ctx, fnl)

	// test list first, the rest are point lookup tests
	t.Run("calls-get", func(t *testing.T) {
		filter := &models.CallFilter{AppID: call.AppID, Path: call.Path, PerPage: 100}
		now := time.Now()
		call.CreatedAt = common.DateTime(now)
		call.ID = id.New().String()
		err := fnl.InsertCall(ctx, call)
		if err != nil {
			t.Fatal(err)
		}
		calls, err := fnl.GetCalls(ctx, filter)
		if err != nil {
			t.Fatalf("Test GetCalls(ctx, filter): unexpected error `%v`", err)
		}
		if len(calls) != 1 {
			t.Fatalf("Test GetCalls(ctx, filter): unexpected length `%v`", len(calls))
		}

		c2 := *call
		c3 := *call
		now = time.Now().Add(100 * time.Millisecond)
		c2.CreatedAt = common.DateTime(now) // add ms cuz db uses it for sort
		c2.ID = id.New().String()

		now = time.Now().Add(200 * time.Millisecond)
		c3.CreatedAt = common.DateTime(now)
		c3.ID = id.New().String()

		err = fnl.InsertCall(ctx, &c2)
		if err != nil {
			t.Fatal(err)
		}
		err = fnl.InsertCall(ctx, &c3)
		if err != nil {
			t.Fatal(err)
		}

		// test that no filter works too
		calls, err = fnl.GetCalls(ctx, &models.CallFilter{AppID: call.AppID, PerPage: 100})
		if err != nil {
			t.Fatalf("Test GetCalls(ctx, filter): unexpected error `%v`", err)
		}
		if len(calls) != 3 {
			t.Fatalf("Test GetCalls(ctx, filter): unexpected length `%v`", len(calls))
		}

		// test that pagination stuff works. id, descending
		filter.PerPage = 1
		calls, err = fnl.GetCalls(ctx, filter)
		if err != nil {
			t.Fatalf("Test GetCalls(ctx, filter): unexpected error `%v`", err)
		}
		if len(calls) != 1 {
			t.Fatalf("Test GetCalls(ctx, filter): unexpected length `%v`", len(calls))
		} else if calls[0].ID != c3.ID {
			t.Fatalf("Test GetCalls: call ids not in expected order: %v %v", calls[0].ID, c3.ID)
		}

		filter.PerPage = 100
		filter.Cursor = calls[0].ID
		calls, err = fnl.GetCalls(ctx, filter)
		if err != nil {
			t.Fatalf("Test GetCalls(ctx, filter): unexpected error `%v`", err)
		}
		if len(calls) != 2 {
			t.Fatalf("Test GetCalls(ctx, filter): unexpected length `%v`", len(calls))
		} else if calls[0].ID != c2.ID {
			t.Fatalf("Test GetCalls: call ids not in expected order: %v %v", calls[0].ID, c2.ID)
		} else if calls[1].ID != call.ID {
			t.Fatalf("Test GetCalls: call ids not in expected order: %v %v", calls[1].ID, call.ID)
		}

		// test that filters actually applied
		calls, err = fnl.GetCalls(ctx, &models.CallFilter{AppID: "wrongappname", PerPage: 100})
		if err != nil {
			t.Fatalf("Test GetCalls(ctx, filter): unexpected error `%v`", err)
		}
		if len(calls) != 0 {
			t.Fatalf("Test GetCalls(ctx, filter): unexpected length `%v`", len(calls))
		}

		calls, err = fnl.GetCalls(ctx, &models.CallFilter{AppID: call.AppID, Path: "wrongpath", PerPage: 100})
		if err != nil {
			t.Fatalf("Test GetCalls(ctx, filter): unexpected error `%v`", err)
		}
		if len(calls) != 0 {
			t.Fatalf("Test GetCalls(ctx, filter): unexpected length `%v`", len(calls))
		}

		// make sure from_time and to_time work
		filter = &models.CallFilter{
			PerPage:  100,
			FromTime: call.CreatedAt,
			ToTime:   c3.CreatedAt,
			AppID:    call.AppID,
		}
		calls, err = fnl.GetCalls(ctx, filter)
		if err != nil {
			t.Fatalf("Test GetCalls(ctx, filter): unexpected error `%v`", err)
		}
		if len(calls) != 1 {
			t.Fatalf("Test GetCalls(ctx, filter): unexpected length `%v`", len(calls))
		} else if calls[0].ID != c2.ID {
			t.Fatalf("Test GetCalls: call id not expected %s vs %s", calls[0].ID, c2.ID)
		}
	})

	t.Run("call-log-insert-get", func(t *testing.T) {
		call.ID = id.New().String()
		logText := "test"
		log := strings.NewReader(logText)
		err := fnl.InsertLog(ctx, call.AppID, call.FnID, call.ID, log)
		if err != nil {
			t.Fatalf("Test InsertLog(ctx, call.ID, logText): unexpected error during inserting log `%v`", err)
		}
		logEntry, err := fnl.GetLog(ctx, call.ID)
		var b bytes.Buffer
		io.Copy(&b, logEntry)
		if !strings.Contains(b.String(), logText) {
			t.Fatalf("Test GetLog(ctx, call.ID, logText): unexpected error, log mismatch. "+
				"Expected: `%v`. Got `%v`.", logText, b.String())
		}
	})

	t.Run("call-log-not-found", func(t *testing.T) {
		call.ID = id.New().String()
		_, err := fnl.GetLog(ctx, call.AppID, call.FnID, call.ID)
		if err != models.ErrCallLogNotFound {
			t.Fatal("GetLog should return not found, but got:", err)
		}
	})

	call = new(models.Call)
	call.CreatedAt = common.DateTime(time.Now())
	call.Status = "error"
	call.Error = "ya dun goofed"
	call.StartedAt = common.DateTime(time.Now())
	call.CompletedAt = common.DateTime(time.Now())
	call.AppID = testApp.Name
	call.Path = testRoute.Path

	t.Run("call-insert", func(t *testing.T) {
		call.ID = id.New().String()
		err := fnl.InsertCall(ctx, call)
		if err != nil {
			t.Fatalf("Test InsertCall(ctx, &call): unexpected error `%v`", err)
		}
	})

	t.Run("call-get", func(t *testing.T) {
		call.ID = id.New().String()
		err := fnl.InsertCall(ctx, call)
		if err != nil {
			t.Fatalf("Test GetCall: unexpected error `%v`", err)
		}
		newCall, err := fnl.GetCall(ctx, call.AppID, call.ID)
		if err != nil {
			t.Fatalf("Test GetCall: unexpected error `%v`", err)
		}
		if call.ID != newCall.ID {
			t.Fatalf("Test GetCall: id mismatch `%v` `%v`", call.ID, newCall.ID)
		}
		if call.Status != newCall.Status {
			t.Fatalf("Test GetCall: status mismatch `%v` `%v`", call.Status, newCall.Status)
		}
		if call.Error != newCall.Error {
			t.Fatalf("Test GetCall: error mismatch `%v` `%v`", call.Error, newCall.Error)
		}
		if time.Time(call.CreatedAt).Unix() != time.Time(newCall.CreatedAt).Unix() {
			t.Fatalf("Test GetCall: created_at mismatch `%v` `%v`", call.CreatedAt, newCall.CreatedAt)
		}
		if time.Time(call.StartedAt).Unix() != time.Time(newCall.StartedAt).Unix() {
			t.Fatalf("Test GetCall: started_at mismatch `%v` `%v`", call.StartedAt, newCall.StartedAt)
		}
		if time.Time(call.CompletedAt).Unix() != time.Time(newCall.CompletedAt).Unix() {
			t.Fatalf("Test GetCall: completed_at mismatch `%v` `%v`", call.CompletedAt, newCall.CompletedAt)
		}
		if call.AppID != newCall.AppID {
			t.Fatalf("Test GetCall: app_name mismatch `%v` `%v`", call.AppID, newCall.AppID)
		}
		if call.Path != newCall.Path {
			t.Fatalf("Test GetCall: path mismatch `%v` `%v`", call.Path, newCall.Path)
		}
	})
}
