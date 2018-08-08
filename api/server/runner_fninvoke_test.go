package server

import (
	"net/http"
	"strings"
	"testing"

	"github.com/fnproject/fn/api/datastore"
	"github.com/fnproject/fn/api/event"
	"github.com/fnproject/fn/api/logs"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/mqs"
)

func TestBadRequests(t *testing.T) {
	buf := setLogBuffer()
	app := &models.App{ID: "app_id", Name: "myapp", Config: models.Config{}}
	fn := &models.Fn{ID: "fn_id", AppID: "app_id"}
	fn2 := &models.Fn{ID: "fn_id2", AppID: "app_id", Format: "cloudevent"}
	ds := datastore.NewMockInit(
		[]*models.App{app},
		[]*models.Fn{fn, fn2},
	)
	rnr, cancel := testRunner(t, ds)
	defer cancel()
	logDB := logs.NewMock()
	srv := testServer(ds, &mqs.Mock{}, logDB, rnr, ServerTypeFull)

	for i, test := range []struct {
		path          string
		contentType   string
		body          string
		expectedCode  int
		expectedError error
	}{
		{"/invoke/notfn", "", "", http.StatusNotFound, models.ErrFnsNotFound},
		{"/invoke/fn_id2", "", "", http.StatusUnsupportedMediaType, models.ErrUnsupportedMediaType},
		{"/invoke/fn_id", fnInvokeContentType, "", http.StatusBadRequest, models.ErrOnlyCloudEventFnsSupported},
		{"/invoke/fn_id2", fnInvokeContentType, "", http.StatusBadRequest, models.ErrInvalidJSON},
		{"/invoke/fn_id2", fnInvokeContentType, "{\"string\": 1}", http.StatusBadRequest, event.ErrEventInvalidEventType},
	} {
		request := createRequest(t, "POST", test.path, strings.NewReader(test.body))
		request.Header = map[string][]string{"Content-Type": []string{test.contentType}}
		_, rec := routerRequest2(t, srv.Router, request)

		if rec.Code != test.expectedCode {
			t.Log(buf.String())
			t.Fatalf("Test %d: Expected status code for path %s to be %d but was %d",
				i, test.path, test.expectedCode, rec.Code)
		}

		if test.expectedError != nil {
			resp := getErrorResponse(t, rec)

			if !strings.Contains(resp.Message, test.expectedError.Error()) {
				t.Log(buf.String())
				t.Errorf("Test %d: Expected error message to have `%s`, but got `%s`",
					i, test.expectedError.Error(), resp.Message)
			}
		}
	}
}
