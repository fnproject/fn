package tests

import (
	"bytes"
	"testing"
	"time"
	"net/url"
	"path"

	"gitlab-odx.oracle.com/odx/functions/fn/client"
	"github.com/funcy/functions_go/client/call"
)

func TestCalls(t *testing.T) {
	s := SetupDefaultSuite()

	t.Run("list-calls-for-missing-app", func(t *testing.T) {
		cfg := &call.GetAppsAppCallsRouteParams{
			App: s.AppName,
			Route: s.RoutePath,
			Context: s.Context,
		}
		_, err := s.Client.Call.GetAppsAppCallsRoute(cfg)
		if err == nil {
			t.Fatalf("Must fail with missing app error, but got %s", err)
		}
	})

	CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})

	u := url.URL{
		Scheme: "http",
		Host:   client.Host(),
	}
	u.Path = path.Join(u.Path, "r", s.AppName, s.RoutePath)

	t.Run("list-calls-for-missing-route", func(t *testing.T) {
		cfg := &call.GetAppsAppCallsRouteParams{
			App: s.AppName,
			Route: s.RoutePath,
			Context: s.Context,
		}
		_, err := s.Client.Call.GetAppsAppCallsRoute(cfg)
		if err == nil {
			t.Fatalf("Must fail with missing route error, but got %s", err)
		}
	})

	CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType,
		s.RouteConfig, s.RouteHeaders)

	t.Run("get-dummy-call", func(t *testing.T) {
		cfg := &call.GetCallsCallParams{
			Call: "dummy",
			Context: s.Context,
		}
		cfg.WithTimeout(time.Second * 60)
		_, err := s.Client.Call.GetCallsCall(cfg)
		if err == nil {
			t.Fatal("Must fail because `dummy` call does not exist.")
		}
		t.Logf("Test `%v` passed.", t.Name())
	})

	t.Run("get-real-call", func(t *testing.T) {

		callID := CallAsync(t, u, &bytes.Buffer{})
		time.Sleep(time.Second * 2)
		cfg := &call.GetCallsCallParams{
			Call: callID,
			Context: s.Context,
		}
		cfg.WithTimeout(time.Second * 60)
		_, err := s.Client.Call.GetCallsCall(cfg)
		if err != nil {
			switch err.(type) {
			case *call.GetCallsCallNotFound:
				msg := err.(*call.GetCallsCallNotFound).Payload.Error.Message
				t.Fatalf("Unexpected error occurred: %v.", msg)
			}
		}
		t.Logf("Test `%v` passed.", t.Name())
	})

	t.Run("list-calls", func(t *testing.T) {
		cfg := &call.GetAppsAppCallsRouteParams{
			App: s.AppName,
			Route: s.RoutePath,
			Context: s.Context,
		}
		calls, err := s.Client.Call.GetAppsAppCallsRoute(cfg)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		if len(calls.Payload.Calls) == 0 {
			t.Fatalf("Must fail. There should be at least one call to `%v` route.", s.RoutePath)
		}
		for _, c := range calls.Payload.Calls {
			if c.Path != s.RoutePath {
				t.Fatalf("Call path mismatch.\n\tExpected: %v\n\tActual: %v", c.Path, s.RoutePath)
			}
		}
	})

	DeleteRoute(t, s.Context, s.Client, s.AppName, s.RoutePath)
	DeleteApp(t, s.Context, s.Client, s.AppName)
}
