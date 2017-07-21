package tests

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"gitlab-odx.oracle.com/odx/functions/api/server"

	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/funcy/functions_go/client"
	"github.com/funcy/functions_go/client/apps"
	"github.com/funcy/functions_go/client/routes"
	"github.com/funcy/functions_go/models"
	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	"github.com/spf13/viper"
)

func Host() string {
	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8080"
	}

	u, err := url.Parse(apiURL)
	if err != nil {
		log.Fatalln("Couldn't parse API URL:", err)
	}
	return u.Host
}

func APIClient() *client.Functions {
	transport := httptransport.New(Host(), "/v1", []string{"http"})
	if os.Getenv("FN_TOKEN") != "" {
		transport.DefaultAuthentication = httptransport.BearerToken(os.Getenv("FN_TOKEN"))
	}

	// create the API client, with the transport
	client := client.New(transport, strfmt.Default)

	return client
}

var (
	getServer sync.Once
)

func getServerWithCancel() (*server.Server, context.CancelFunc) {
	var cancel2 context.CancelFunc
	var s *server.Server
	getServer.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())

		viper.Set(server.EnvPort, "8080")
		viper.Set(server.EnvAPIURL, "http://localhost:8080")
		viper.Set(server.EnvLogLevel, "fatal")
		timeString := time.Now().Format("2006_01_02_15_04_05")
		tmpDir := os.TempDir()
		tmpMq := fmt.Sprintf("%s/fn_integration_test_%s_worker_mq.db", tmpDir, timeString)
		tmpDB := fmt.Sprintf("%s/fn_integration_test_%s_fn.db", tmpDir, timeString)
		viper.Set(server.EnvMQURL, fmt.Sprintf("bolt://%s", tmpMq))
		viper.Set(server.EnvDBURL, fmt.Sprintf("sqlite3://%s", tmpDB))

		s = server.NewFromEnv(ctx)

		go s.Start(ctx)
		started := false
		time.AfterFunc(time.Second*10, func() {
			if !started {
				panic("Failed to start server.")
			}
		})
		_, err := http.Get(viper.GetString(server.EnvAPIURL) + "/version")
		for err != nil {
			_, err = http.Get(viper.GetString(server.EnvAPIURL) + "/version")
		}
		started = true
		cancel2 = context.CancelFunc(func() {
			cancel()
			os.Remove(tmpMq)
			os.Remove(tmpDB)
		})
	})
	return s, cancel2
}

type SuiteSetup struct {
	Context      context.Context
	Client       *client.Functions
	AppName      string
	RoutePath    string
	Image        string
	RouteType    string
	Format       string
	Memory       int64
	RouteConfig  map[string]string
	RouteHeaders map[string][]string
	Cancel       context.CancelFunc
}

func SetupDefaultSuite() *SuiteSetup {
	ss := &SuiteSetup{
		Context:      context.Background(),
		Client:       APIClient(),
		AppName:      "test-app",
		RoutePath:    "/hello",
		Image:        "funcy/hello",
		Format:       "default",
		RouteType:    "async",
		RouteConfig:  map[string]string{},
		RouteHeaders: map[string][]string{},
		Cancel:       func() {},
	}

	_, ok := ss.Client.Version.GetVersion(nil)
	if ok != nil {
		if Host() != "localhost:8080" {
			_, ok := http.Get(fmt.Sprintf("http://%s/version", Host()))
			if ok != nil {
				panic("Cannot reach remote api for functions")
			}
		} else {
			_, ok := http.Get(fmt.Sprintf("http://%s/version", Host()))
			if ok != nil {
				log.Println("Making functions server")
				_, cancel := getServerWithCancel()
				ss.Cancel = cancel
			}
		}
	}
	return ss
}

func CheckAppResponseError(t *testing.T, err error) {
	if err != nil {
		switch err.(type) {

		case *apps.DeleteAppsAppDefault:
			msg := err.(*apps.DeleteAppsAppDefault).Payload.Error.Message
			code := err.(*apps.DeleteAppsAppDefault).Code()
			t.Fatalf("Unexpected error occurred: %v. Status code: %v", msg, code)
			return

		case *apps.PostAppsDefault:
			msg := err.(*apps.PostAppsDefault).Payload.Error.Message
			code := err.(*apps.PostAppsDefault).Code()
			t.Fatalf("Unexpected error occurred: %v. Status code: %v", msg, code)
			return

		case *apps.GetAppsAppNotFound:
			msg := err.(*apps.GetAppsAppNotFound).Payload.Error.Message
			if !strings.Contains("App not found", msg) {
				t.Fatalf("Unexpected error occurred: %v", msg)
				return
			}
			return

		case *apps.GetAppsAppDefault:
			msg := err.(*apps.GetAppsAppDefault).Payload.Error.Message
			code := err.(*apps.GetAppsAppDefault).Code()
			t.Fatalf("Unexpected error occurred: %v. Status code: %v", msg, code)
			return

		case *apps.PatchAppsAppDefault:
			msg := err.(*apps.PatchAppsAppDefault).Payload.Error.Message
			code := err.(*apps.PatchAppsAppDefault).Code()
			t.Fatalf("Unexpected error occurred: %v. Status code: %v", msg, code)
			return

		case *apps.PatchAppsAppNotFound:
			msg := err.(*apps.PatchAppsAppNotFound).Payload.Error.Message
			t.Fatalf("Unexpected error occurred: %v.", msg)
			return

		case *apps.PatchAppsAppBadRequest:
			msg := err.(*apps.PatchAppsAppBadRequest).Payload.Error.Message
			t.Fatalf("Unexpected error occurred: %v.", msg)
			return
		}
		t.Fatalf("Unable to determine type of error: %s", err)
	}

}

func CreateAppNoAssert(ctx context.Context, fnclient *client.Functions, appName string, config map[string]string) (*apps.PostAppsOK, error) {
	cfg := &apps.PostAppsParams{
		Body: &models.AppWrapper{
			App: &models.App{
				Config: config,
				Name:   appName,
			},
		},
		Context: ctx,
	}
	cfg.WithTimeout(time.Second * 60)
	return fnclient.Apps.PostApps(cfg)
}

func CreateApp(t *testing.T, ctx context.Context, fnclient *client.Functions, appName string, config map[string]string) {
	appPayload, err := CreateAppNoAssert(ctx, fnclient, appName, config)
	CheckAppResponseError(t, err)
	if !strings.Contains(appName, appPayload.Payload.App.Name) {
		t.Fatalf("App name mismatch.\nExpected: %v\nActual: %v",
			appName, appPayload.Payload.App.Name)
	}
}

func UpdateApp(t *testing.T, ctx context.Context, fnclient *client.Functions, appName string, config map[string]string) *apps.PatchAppsAppOK {
	CreateApp(t, ctx, fnclient, appName, map[string]string{"A": "a"})
	cfg := &apps.PatchAppsAppParams{
		App: appName,
		Body: &models.AppWrapper{
			App: &models.App{
				Config: config,
				Name:   "",
			},
		},
		Context: ctx,
	}
	appPayload, err := fnclient.Apps.PatchAppsApp(cfg)
	CheckAppResponseError(t, err)
	return appPayload
}

func DeleteApp(t *testing.T, ctx context.Context, fnclient *client.Functions, appName string) {
	cfg := &apps.DeleteAppsAppParams{
		App:     appName,
		Context: ctx,
	}
	cfg.WithTimeout(time.Second * 60)
	_, err := fnclient.Apps.DeleteAppsApp(cfg)
	CheckAppResponseError(t, err)
}

func CheckRouteResponseError(t *testing.T, err error) {
	if err != nil {
		switch err.(type) {

		case *routes.PostAppsAppRoutesDefault:
			msg := err.(*routes.PostAppsAppRoutesDefault).Payload.Error.Message
			code := err.(*routes.PostAppsAppRoutesDefault).Code()
			t.Fatalf("Unexpected error occurred: %v. Status code: %v", msg, code)
			return

		case *routes.PostAppsAppRoutesBadRequest:
			msg := err.(*routes.PostAppsAppRoutesBadRequest).Payload.Error.Message
			t.Fatalf("Unexpected error occurred: %v.", msg)
			return

		case *routes.PostAppsAppRoutesConflict:
			msg := err.(*routes.PostAppsAppRoutesConflict).Payload.Error.Message
			t.Fatalf("Unexpected error occurred: %v.", msg)
			return

		case *routes.GetAppsAppRoutesRouteNotFound:
			msg := err.(*routes.GetAppsAppRoutesRouteNotFound).Payload.Error.Message
			t.Fatalf("Unexpected error occurred: %v.", msg)
			return

		case *routes.GetAppsAppRoutesRouteDefault:
			msg := err.(*routes.GetAppsAppRoutesRouteDefault).Payload.Error.Message
			code := err.(*routes.GetAppsAppRoutesRouteDefault).Code()
			t.Fatalf("Unexpected error occurred: %v. Status code: %v", msg, code)
			return

		case *routes.DeleteAppsAppRoutesRouteNotFound:
			msg := err.(*routes.DeleteAppsAppRoutesRouteNotFound).Payload.Error.Message
			t.Fatalf("Unexpected error occurred: %v.", msg)
			return

		case *routes.DeleteAppsAppRoutesRouteDefault:
			msg := err.(*routes.DeleteAppsAppRoutesRouteDefault).Payload.Error.Message
			code := err.(*routes.DeleteAppsAppRoutesRouteDefault).Code()
			t.Fatalf("Unexpected error occurred: %v. Status code: %v", msg, code)
			return
		case *routes.GetAppsAppRoutesNotFound:
			msg := err.(*routes.GetAppsAppRoutesNotFound).Payload.Error.Message
			t.Fatalf("Unexpected error occurred: %v.", msg)
			return

		case *routes.GetAppsAppRoutesDefault:
			msg := err.(*routes.GetAppsAppRoutesDefault).Payload.Error.Message
			code := err.(*routes.GetAppsAppRoutesDefault).Code()
			t.Fatalf("Unexpected error occurred: %v. Status code: %v", msg, code)
			return

		case *routes.PatchAppsAppRoutesRouteBadRequest:
			msg := err.(*routes.PatchAppsAppRoutesRouteBadRequest).Payload.Error.Message
			t.Fatalf("Unexpected error occurred: %v.", msg)
			return

		case *routes.PatchAppsAppRoutesRouteNotFound:
			msg := err.(*routes.PatchAppsAppRoutesRouteNotFound).Payload.Error.Message
			t.Fatalf("Unexpected error occurred: %v.", msg)
			return

		case *routes.PatchAppsAppRoutesRouteDefault:
			msg := err.(*routes.PatchAppsAppRoutesRouteDefault).Payload.Error.Message
			code := err.(*routes.PatchAppsAppRoutesRouteDefault).Code()
			t.Fatalf("Unexpected error occurred: %v. Status code: %v", msg, code)
			return

		}
		t.Fatalf("Unable to determine type of error: %s", err)
	}
}

func logRoute(t *testing.T, routeObject *models.Route) {
	t.Logf("Route path: %v", routeObject.Path)
	t.Logf("Route image: %v", routeObject.Image)
	t.Logf("Route type: %v", routeObject.Type)
	t.Logf("Route timeout: %vs", *routeObject.Timeout)
	t.Logf("Route idle timeout: %vs", *routeObject.IDLETimeout)
}

func assertRouteFields(t *testing.T, routeObject *models.Route, path, image, routeType string) {

	logRoute(t, routeObject)
	rPath := routeObject.Path
	rImage := routeObject.Image
	rType := routeObject.Type
	rTimeout := *routeObject.Timeout
	rIdleTimeout := *routeObject.IDLETimeout
	if rPath != path {
		t.Fatalf("Route path mismatch. Expected: %v. Actual: %v", path, rPath)
	}
	if rImage != image {
		t.Fatalf("Route image mismatch. Expected: %v. Actual: %v", image, rImage)
	}
	if rType != routeType {
		t.Fatalf("Route type mismatch. Expected: %v. Actual: %v", routeType, rType)
	}
	if rTimeout == 0 {
		t.Fatal("Route timeout should have default value of 30 seconds, but got 0 seconds")
	}
	if rIdleTimeout == 0 {
		t.Fatal("Route idle timeout should have default value of 30 seconds, but got 0 seconds")
	}

}

func createRoute(ctx context.Context, fnclient *client.Functions, appName, image, routePath, routeType string, routeConfig map[string]string, headers map[string][]string) (*routes.PostAppsAppRoutesOK, error) {
	cfg := &routes.PostAppsAppRoutesParams{
		App: appName,
		Body: &models.RouteWrapper{
			Route: &models.Route{
				Config:  routeConfig,
				Headers: headers,
				Image:   image,
				Path:    routePath,
				Type:    routeType,
			},
		},
		Context: ctx,
	}
	cfg.WithTimeout(time.Second * 60)
	return fnclient.Routes.PostAppsAppRoutes(cfg)

}

func CreateRoute(t *testing.T, ctx context.Context, fnclient *client.Functions, appName, routePath, image, routeType string, routeConfig map[string]string, headers map[string][]string) {
	routeResponse, err := createRoute(ctx, fnclient, appName, image, routePath, routeType, routeConfig, headers)
	CheckRouteResponseError(t, err)

	assertRouteFields(t, routeResponse.Payload.Route, routePath, image, routeType)
}

func deleteRoute(ctx context.Context, fnclient *client.Functions, appName, routePath string) (*routes.DeleteAppsAppRoutesRouteOK, error) {
	cfg := &routes.DeleteAppsAppRoutesRouteParams{
		App:     appName,
		Route:   routePath,
		Context: ctx,
	}
	cfg.WithTimeout(time.Second * 60)
	return fnclient.Routes.DeleteAppsAppRoutesRoute(cfg)
}

func DeleteRoute(t *testing.T, ctx context.Context, fnclient *client.Functions, appName, routePath string) {
	_, err := deleteRoute(ctx, fnclient, appName, routePath)
	CheckRouteResponseError(t, err)
}

func ListRoutes(t *testing.T, ctx context.Context, fnclient *client.Functions, appName string) []*models.Route {
	cfg := &routes.GetAppsAppRoutesParams{
		App:     appName,
		Context: ctx,
	}
	cfg.WithTimeout(time.Second * 60)
	routesResponse, err := fnclient.Routes.GetAppsAppRoutes(cfg)
	CheckRouteResponseError(t, err)
	return routesResponse.Payload.Routes
}

func GetRoute(t *testing.T, ctx context.Context, fnclient *client.Functions, appName, routePath string) *models.Route {
	cfg := &routes.GetAppsAppRoutesRouteParams{
		App:     appName,
		Route:   routePath,
		Context: ctx,
	}
	cfg.WithTimeout(time.Second * 60)
	routeResponse, err := fnclient.Routes.GetAppsAppRoutesRoute(cfg)
	CheckRouteResponseError(t, err)
	return routeResponse.Payload.Route
}

func UpdateRoute(t *testing.T, ctx context.Context, fnclient *client.Functions, appName, routePath, image, routeType, format string, memory int64, routeConfig map[string]string, headers map[string][]string, newRoutePath string) (*routes.PatchAppsAppRoutesRouteOK, error) {

	routeObject := GetRoute(t, ctx, fnclient, appName, routePath)
	if routeObject.Config == nil {
		routeObject.Config = map[string]string{}
	}

	if routeObject.Headers == nil {
		routeObject.Headers = map[string][]string{}
	}
	logRoute(t, routeObject)

	routeObject.Path = ""
	if newRoutePath != "" {
		routeObject.Path = newRoutePath
	}

	if routeConfig != nil {
		for k, v := range routeConfig {
			if string(k[0]) == "-" {
				delete(routeObject.Config, string(k[1:]))
				continue
			}
			routeObject.Config[k] = v
		}
	}
	if headers != nil {
		for k, v := range headers {
			if string(k[0]) == "-" {
				delete(routeObject.Headers, k)
				continue
			}
			routeObject.Headers[k] = v
		}
	}
	if image != "" {
		routeObject.Image = image
	}
	if format != "" {
		routeObject.Format = format
	}
	if routeType != "" {
		routeObject.Type = routeType
	}
	if memory > 0 {
		routeObject.Memory = memory
	}

	cfg := &routes.PatchAppsAppRoutesRouteParams{
		App:     appName,
		Context: ctx,
		Body: &models.RouteWrapper{
			Route: routeObject,
		},
		Route: routePath,
	}
	cfg.WithTimeout(time.Second * 60)

	t.Log("Calling update")

	return fnclient.Routes.PatchAppsAppRoutesRoute(cfg)
}

func assertContainsRoute(routeModels []*models.Route, expectedRoute string) bool {
	for _, r := range routeModels {
		if r.Path == expectedRoute {
			return true
		}
	}
	return false
}

func EnvAsHeader(req *http.Request, selectedEnv []string) {
	detectedEnv := os.Environ()
	if len(selectedEnv) > 0 {
		detectedEnv = selectedEnv
	}

	for _, e := range detectedEnv {
		kv := strings.Split(e, "=")
		name := kv[0]
		req.Header.Set(name, os.Getenv(name))
	}
}

func CallFN(u string, content io.Reader, output io.Writer, method string, env []string) error {
	if method == "" {
		if content == nil {
			method = "GET"
		} else {
			method = "POST"
		}
	}

	req, err := http.NewRequest(method, u, content)
	if err != nil {
		return fmt.Errorf("error running route: %s", err)
	}

	req.Header.Set("Content-Type", "application/json")

	if len(env) > 0 {
		EnvAsHeader(req, env)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error running route: %s", err)
	}

	io.Copy(output, resp.Body)

	return nil
}
