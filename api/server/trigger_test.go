package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fnproject/fn/api/datastore"
	"github.com/fnproject/fn/api/logs"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/mqs"
)

func TestTriggerCreate(t *testing.T) {
	buf := setLogBuffer()
	defer func() {
		if t.Failed() {
			t.Log(buf.String())
		}
	}()

	a := &models.App{ID: "appid"}
	a.SetDefaults()

	fn := &models.Fn{ID: "fnid"}
	fn.SetDefaults()
	commonDS := datastore.NewMockInit([]*models.App{a}, []*models.Fn{fn})

	for i, test := range []struct {
		mock          models.Datastore
		logDB         models.LogStore
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		// errors
		{commonDS, logs.NewMock(), "/v2/triggers", ``, http.StatusBadRequest, models.ErrInvalidJSON},
		{commonDS, logs.NewMock(), "/v2/triggers", `{}`, http.StatusBadRequest, models.ErrTriggerMissingName},

		{commonDS, logs.NewMock(), "/v2/triggers", `{ "name": "Test" }`, http.StatusBadRequest, models.ErrTriggerMissingAppID},

		{commonDS, logs.NewMock(), "/v2/triggers", `{ "name": "1234567890123456789012345678901" } }`, http.StatusBadRequest, models.ErrTriggerTooLongName},
		{commonDS, logs.NewMock(), "/v2/triggers", `{ "name": "&&%@!#$#@$" } }`, http.StatusBadRequest, models.ErrTriggerInvalidName},
		{commonDS, logs.NewMock(), "/v2/triggers", `{ "name": "trigger", "app_id": "appid", "fn_id": "fnid", "type": "HTTP", "source": "src", "annotations" : { "":"val" }}`, http.StatusBadRequest, models.ErrInvalidAnnotationKey},

		// // success
		{commonDS, logs.NewMock(), "/v2/triggers", `{ "name": "trigger", "app_id": "appid", "fn_id": "fnid", "type": "HTTP", "source": "src"}`, http.StatusOK, nil},
	} {
		rnr, cancel := testRunner(t)
		srv := testServer(test.mock, &mqs.Mock{}, test.logDB, rnr, ServerTypeFull)
		router := srv.Router

		body := bytes.NewBuffer([]byte(test.body))
		_, rec := routerRequest(t, router, "PUT", test.path, body)

		if rec.Code != test.expectedCode {
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		if test.expectedError != nil {
			resp := getErrorResponse(t, rec)

			if !strings.Contains(resp.Error.Message, test.expectedError.Error()) {
				t.Errorf("Test %d: Expected error message to have `%s` but got `%s`",
					i, test.expectedError.Error(), resp.Error.Message)
			}
		}

		if test.expectedCode == http.StatusOK {
			var triggerResp triggerResponse
			err := json.NewDecoder(rec.Body).Decode(&triggerResp)
			if err != nil {
				t.Log(buf.String())
				t.Errorf("Test %d: error decoding body for 'ok' json, it was a lie: %v", i, err)
			}

			trigger := triggerResp.Trigger

			// IsZero() doesn't really work, this ensures it's not unset as long as we're not in 1970
			if time.Time(trigger.CreatedAt).Before(time.Now().Add(-1 * time.Hour)) {
				t.Log(buf.String())
				t.Errorf("Test %d: expected created_at to be set on trigger, it wasn't: %s", i, trigger.CreatedAt)
			}
			if !(time.Time(trigger.CreatedAt)).Equal(time.Time(trigger.UpdatedAt)) {
				t.Log(buf.String())
				t.Errorf("Test %d: expected updated_at to be set and same as created at, it wasn't: %s %s", i, trigger.CreatedAt, trigger.UpdatedAt)
			}
		}

		cancel()
	}
}
