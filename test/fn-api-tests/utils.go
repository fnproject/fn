package tests

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fnproject/fn/api/server"
	"github.com/fnproject/fn_go/client"
	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
)

const lBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func GetAPIURL() (string, *url.URL) {
	apiURL := getEnv("FN_API_URL", "http://localhost:8080")
	u, err := url.Parse(apiURL)
	if err != nil {
		log.Fatalf("Couldn't parse API URL: %s error: %s", apiURL, err)
	}
	return apiURL, u
}

func Host() string {
	_, u := GetAPIURL()
	return u.Host
}

func APIClient() *client.Fn {
	transport := httptransport.New(Host(), "/v1", []string{"http"})
	if os.Getenv("FN_TOKEN") != "" {
		transport.DefaultAuthentication = httptransport.BearerToken(os.Getenv("FN_TOKEN"))
	}

	// create the API client, with the transport
	return client.New(transport, strfmt.Default)
}

func checkServer(ctx context.Context) error {
	if ctx.Err() != nil {
		log.Print("Server check failed, timeout")
		return ctx.Err()
	}

	apiURL, _ := GetAPIURL()

	client := &http.Client{}
	req, err := http.NewRequest("GET", apiURL+"/version", nil)
	if err != nil {
		log.Panicf("Server check new request failed: %s", err)
	}

	req = req.WithContext(ctx)
	_, err = client.Do(req)
	if err != nil {
		log.Printf("Server is not up... err: %s", err)
		return err
	}
	return ctx.Err()
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

var (
	srvLock     sync.Mutex
	srvRefCount uint64
	srvInstance *server.Server
	srvDone     chan struct{}
	srvCancel   func()
)

func stopServer(ctx context.Context) {
	srvLock.Lock()
	defer srvLock.Unlock()
	if srvRefCount == 0 {
		log.Printf("Server not running, ref count %v", srvRefCount)
		return
	}

	srvRefCount--
	if srvRefCount != 0 {
		log.Printf("Server decrement ref count %v", srvRefCount)
		return
	}

	srvCancel()

	select {
	case <-srvDone:
	case <-ctx.Done():
		log.Panic("Server Cleanup failed, timeout")
	}
}

func startServer() {

	srvLock.Lock()
	srvRefCount++

	if srvRefCount != 1 {
		log.Printf("Server already running, ref count %v", srvRefCount)
		srvLock.Unlock()

		// check once
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Duration(2)*time.Second))
		defer cancel()
		err := checkServer(ctx)
		if err != nil {
			log.Panicf("Server check failed: %s", err)
		}

		return
	}

	log.Printf("Starting server, ref count %v", srvRefCount)

	srvDone = make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	srvCancel = cancel

	timeString := time.Now().Format("2006_01_02_15_04_05")
	dbURL := os.Getenv(server.EnvDBURL)
	tmpDir := os.TempDir()
	tmpMq := fmt.Sprintf("%s/fn_integration_test_%s_worker_mq.db", tmpDir, timeString)
	tmpDb := fmt.Sprintf("%s/fn_integration_test_%s_fn.db", tmpDir, timeString)
	mqURL := fmt.Sprintf("bolt://%s", tmpMq)
	if dbURL == "" {
		dbURL = fmt.Sprintf("sqlite3://%s", tmpDb)
	}

	srvInstance = server.New(ctx,
		server.WithLogLevel(getEnv(server.EnvLogLevel, server.DefaultLogLevel)),
		server.WithDBURL(dbURL),
		server.WithMQURL(mqURL),
		server.WithFullAgent(),
	)

	go func() {
		srvInstance.Start(ctx)
		log.Print("Stopped server")
		os.Remove(tmpMq)
		os.Remove(tmpDb)
		close(srvDone)
	}()

	srvLock.Unlock()

	startCtx, startCancel := context.WithDeadline(ctx, time.Now().Add(time.Duration(10)*time.Second))
	defer startCancel()
	for {
		err := checkServer(startCtx)
		if err == nil {
			break
		}
		select {
		case <-time.After(time.Second * 1):
		case <-ctx.Done():
		}
		if ctx.Err() != nil {
			log.Panic("Server check failed, timeout")
		}
	}
}

// TestHarness provides context and pre-configured clients to an individual test, it has some helper functions to create Apps and Routes that mirror the underlying client operations and clean them up after the test is complete
// This is not goroutine safe and each test case should use its own harness.
type TestHarness struct {
	Context      context.Context
	Cancel       func()
	Client       *client.Fn
	AppName      string
	RoutePath    string
	Image        string
	RouteType    string
	Format       string
	Memory       uint64
	Timeout      int32
	IdleTimeout  int32
	RouteConfig  map[string]string
	RouteHeaders map[string][]string

	createdApps map[string]bool
}

func RandStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = lBytes[rand.Intn(len(lBytes))]
	}
	return strings.ToLower(string(b))
}

// SetupHarness creates a test harness for a test case - this picks up external options and
func SetupHarness() *TestHarness {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)

	startServer()

	ss := &TestHarness{
		Context:      ctx,
		Cancel:       cancel,
		Client:       APIClient(),
		AppName:      "fnintegrationtestapp" + RandStringBytes(10),
		RoutePath:    "/fnintegrationtestroute" + RandStringBytes(10),
		Image:        "fnproject/hello",
		Format:       "default",
		RouteType:    "async",
		RouteConfig:  map[string]string{},
		RouteHeaders: map[string][]string{},
		Memory:       uint64(256),
		Timeout:      int32(30),
		IdleTimeout:  int32(30),
		createdApps:  make(map[string]bool),
	}
	return ss
}

func (s *TestHarness) Cleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	//for _,ar := range s.createdRoutes {
	//	deleteRoute(ctx, s.Client, ar.appName, ar.routeName)
	//}

	for app, _ := range s.createdApps {
		safeDeleteApp(ctx, s.Client, app)
	}

	stopServer(ctx)
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

func CallFN(u string, content io.Reader, output io.Writer, method string, env []string) (*http.Response, error) {
	if method == "" {
		if content == nil {
			method = "GET"
		} else {
			method = "POST"
		}
	}

	req, err := http.NewRequest(method, u, content)
	if err != nil {
		return nil, fmt.Errorf("error running route: %s", err)
	}

	req.Header.Set("Content-Type", "application/json")

	if len(env) > 0 {
		EnvAsHeader(req, env)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error running route: %s", err)
	}

	io.Copy(output, resp.Body)

	return resp, nil
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func APICallWithRetry(t *testing.T, attempts int, sleep time.Duration, callback func() error) (err error) {
	for i := 0; i < attempts; i++ {
		err = callback()
		if err == nil {
			t.Log("Exiting retry loop, API call was successful")
			return nil
		}
		t.Logf("[%v] - Retrying API call after unsuccessful attempt with error: %v", i, err.Error())
		time.Sleep(sleep)
	}
	return err
}
