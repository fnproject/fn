package tests

import (
	"bytes"
	"net/url"
	"path"
	"testing"
	"time"

	"github.com/fnproject/fn_go/client/call"
)

func TestCallsMissingApp(t *testing.T) {
	t.Parallel()
	s := SetupDefaultSuite()
	cfg := &call.GetAppsAppCallsParams{
		App:     s.AppName,
		Path:    &s.RoutePath,
		Context: s.Context,
	}
	_, err := s.Client.Call.GetAppsAppCalls(cfg)
	if err == nil {
		t.Errorf("Must fail with missing app error, but got %s", err)
	}
}

func TestCallsDummy(t *testing.T) {
	t.Parallel()
	s := SetupDefaultSuite()
	CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
	CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType,
		s.Format, s.Timeout, s.IdleTimeout, s.RouteConfig, s.RouteHeaders)

	cfg := &call.GetAppsAppCallsCallParams{
		Call:    "dummy",
		App:     s.AppName,
		Context: s.Context,
	}
	cfg.WithTimeout(time.Second * 60)
	_, err := s.Client.Call.GetAppsAppCallsCall(cfg)
	if err == nil {
		t.Error("Must fail because `dummy` call does not exist.")
	}

	DeleteApp(t, s.Context, s.Client, s.AppName)
}

func TestGetCallsSuccess(t *testing.T) {
	t.Parallel()
	s := SetupDefaultSuite()
	CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
	CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType,
		s.Format, s.Timeout, s.IdleTimeout, s.RouteConfig, s.RouteHeaders)

	u := url.URL{
		Scheme: "http",
		Host:   Host(),
	}
	u.Path = path.Join(u.Path, "r", s.AppName, s.RoutePath)

	cfg := &call.GetAppsAppCallsParams{
		App:  s.AppName,
		Path: &s.RoutePath,
	}
	retryErr := APICallWithRetry(t, 10, time.Second*2, func() (err error) {
		_, err = s.Client.Call.GetAppsAppCalls(cfg)
		return err
	})

	if retryErr != nil {
		t.Error(retryErr.Error())
	}

	DeleteApp(t, s.Context, s.Client, s.AppName)
}

func TestListCallsSuccess(t *testing.T) {
	t.Parallel()
	s := SetupDefaultSuite()
	CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
	CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType,
		s.Format, s.Timeout, s.IdleTimeout, s.RouteConfig, s.RouteHeaders)

	u := url.URL{
		Scheme: "http",
		Host:   Host(),
	}
	u.Path = path.Join(u.Path, "r", s.AppName, s.RoutePath)

	CallAsync(t, u, &bytes.Buffer{})

	cfg := &call.GetAppsAppCallsParams{
		App:     s.AppName,
		Path:    &s.RoutePath,
		Context: s.Context,
	}

	retryErr := APICallWithRetry(t, 10, time.Second*2, func() (err error) {
		_, err = s.Client.Call.GetAppsAppCalls(cfg)
		return err
	})

	if retryErr != nil {
		t.Error(retryErr.Error())
	} else {
		calls, err := s.Client.Call.GetAppsAppCalls(cfg)
		if err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if calls == nil || calls.Payload == nil || calls.Payload.Calls == nil || len(calls.Payload.Calls) == 0 {
			t.Errorf("Must fail. There should be at least one call to `%v` route.", s.RoutePath)
			return
		}
		for _, c := range calls.Payload.Calls {
			if c.Path != s.RoutePath {
				t.Errorf("Call path mismatch.\n\tExpected: %v\n\tActual: %v", c.Path, s.RoutePath)
			}
		}
	}

	DeleteApp(t, s.Context, s.Client, s.AppName)
}
