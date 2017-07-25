package tests

import (
	"bytes"
	"net/url"
	"path"
	"testing"
	"time"

	"github.com/funcy/functions_go/client/call"
)

func TestCalls(t *testing.T) {

	t.Run("list-calls-for-missing-app", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		cfg := &call.GetAppsAppCallsRouteParams{
			App:     s.AppName,
			Route:   s.RoutePath,
			Context: s.Context,
		}
		_, err := s.Client.Call.GetAppsAppCallsRoute(cfg)
		if err == nil {
			t.Errorf("Must fail with missing app error, but got %s", err)
		}
	})

	t.Run("list-calls-for-missing-route", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})

		cfg := &call.GetAppsAppCallsRouteParams{
			App:     s.AppName,
			Route:   s.RoutePath,
			Context: s.Context,
		}
		_, err := s.Client.Call.GetAppsAppCallsRoute(cfg)
		if err == nil {
			t.Errorf("Must fail with missing route error, but got %s", err)
		}

		DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("get-dummy-call", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType,
			s.RouteConfig, s.RouteHeaders)

		cfg := &call.GetCallsCallParams{
			Call:    "dummy",
			Context: s.Context,
		}
		cfg.WithTimeout(time.Second * 60)
		_, err := s.Client.Call.GetCallsCall(cfg)
		if err == nil {
			t.Error("Must fail because `dummy` call does not exist.")
		}

		DeleteRoute(t, s.Context, s.Client, s.AppName, s.RoutePath)
		DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("get-real-call", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType,
			s.RouteConfig, s.RouteHeaders)

		u := url.URL{
			Scheme: "http",
			Host:   Host(),
		}
		u.Path = path.Join(u.Path, "r", s.AppName, s.RoutePath)

		callID := CallAsync(t, u, &bytes.Buffer{})
		time.Sleep(time.Second * 5)
		cfg := &call.GetCallsCallParams{
			Call:    callID,
			Context: s.Context,
		}
		cfg.WithTimeout(time.Second * 60)
		_, err := s.Client.Call.GetCallsCall(cfg)
		if err != nil {
			switch err.(type) {
			case *call.GetCallsCallNotFound:
				msg := err.(*call.GetCallsCallNotFound).Payload.Error.Message
				t.Errorf("Unexpected error occurred: %v.", msg)
			}
		}
		DeleteRoute(t, s.Context, s.Client, s.AppName, s.RoutePath)
		DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("list-calls", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType,
			s.RouteConfig, s.RouteHeaders)

		u := url.URL{
			Scheme: "http",
			Host:   Host(),
		}
		u.Path = path.Join(u.Path, "r", s.AppName, s.RoutePath)

		CallAsync(t, u, &bytes.Buffer{})
		time.Sleep(time.Second * 8)

		cfg := &call.GetAppsAppCallsRouteParams{
			App:     s.AppName,
			Route:   s.RoutePath,
			Context: s.Context,
		}
		calls, err := s.Client.Call.GetAppsAppCallsRoute(cfg)
		if err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if len(calls.Payload.Calls) == 0 {
			t.Errorf("Must fail. There should be at least one call to `%v` route.", s.RoutePath)
		}
		for _, c := range calls.Payload.Calls {
			if c.Path != s.RoutePath {
				t.Errorf("Call path mismatch.\n\tExpected: %v\n\tActual: %v", c.Path, s.RoutePath)
			}
		}
		DeleteRoute(t, s.Context, s.Client, s.AppName, s.RoutePath)
		DeleteApp(t, s.Context, s.Client, s.AppName)
	})

}
