package tests

import (
    "bytes"
    //"fmt"
    "net/url"
    //"os"
    "path"
    "strings"
    "testing"

    apiutils "github.com/fnproject/fn/test/fn-api-tests"
)

func LB() (string, error) {
    lbURL := "http://127.0.0.1:8081"

    u, err := url.Parse(lbURL)
    if err != nil {
        return "", err
    }
    return u.Host, nil
}

func TestCanExecuteFunction(t *testing.T) {
    s := apiutils.SetupDefaultSuite()
    apiutils.CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
    apiutils.CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, "sync",
        s.Format, s.Timeout, s.IdleTimeout, s.RouteConfig, s.RouteHeaders)

    lb, err := LB()
    if err != nil {
        t.Fatalf("Got unexpected error: %v", err)
    }
    u := url.URL{
        Scheme: "http",
        Host:   lb,
    }
    u.Path = path.Join(u.Path, "r", s.AppName, s.RoutePath)

    content := &bytes.Buffer{}
    output := &bytes.Buffer{}
    _, err = apiutils.CallFN(u.String(), content, output, "POST", []string{})
    if err != nil {
        t.Errorf("Got unexpected error: %v", err)
    }
    expectedOutput := "Hello World!\n"
    if !strings.Contains(expectedOutput, output.String()) {
        t.Errorf("Assertion error.\n\tExpected: %v\n\tActual: %v", expectedOutput, output.String())
    }
    apiutils.DeleteApp(t, s.Context, s.Client, s.AppName)
}
