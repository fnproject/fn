package test

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"bytes"
	"encoding/json"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/server"
	"github.com/fnproject/fn_go/client"
	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	"testing"
)

const lBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func Host() string {
	apiURL := os.Getenv("FN_API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8080"
	}

	u, err := url.Parse(apiURL)
	if err != nil {
		log.Fatalln("Couldn't parse API URL:", err)
	}
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

var (
	getServer     sync.Once
	cancel2       context.CancelFunc
	s             *server.Server
	appsandroutes = make(map[string][]string)
	approutesLock sync.Mutex
)

func getServerWithCancel() (*server.Server, context.CancelFunc) {
	getServer.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())

		apiURL := "http://localhost:8080"

		common.SetLogLevel("fatal")
		timeString := time.Now().Format("2006_01_02_15_04_05")
		dbURL := os.Getenv(server.EnvDBURL)
		tmpDir := os.TempDir()
		tmpMq := fmt.Sprintf("%s/fn_integration_test_%s_worker_mq.db", tmpDir, timeString)
		tmpDb := fmt.Sprintf("%s/fn_integration_test_%s_fn.db", tmpDir, timeString)
		mqURL := fmt.Sprintf("bolt://%s", tmpMq)
		if dbURL == "" {
			dbURL = fmt.Sprintf("sqlite3://%s", tmpDb)
		}

		s = server.NewFromURLs(ctx, dbURL, mqURL, "")

		go s.Start(ctx)
		started := false
		time.AfterFunc(time.Second*10, func() {
			if !started {
				panic("Failed to start server.")
			}
		})
		log.Println("apiURL:", apiURL)
		_, err := http.Get(apiURL + "/version")
		for err != nil {
			_, err = http.Get(apiURL + "/version")
		}
		started = true
		cancel2 = context.CancelFunc(func() {
			cancel()
			os.Remove(tmpMq)
			os.Remove(tmpDb)
		})
	})
	return s, cancel2
}

type SuiteSetup struct {
	Context      context.Context
	Client       *client.Fn
	AppName      string
	RoutePath    string
	Image        string
	RouteType    string
	Format       string
	Memory       uint64
	RouteConfig  map[string]string
	RouteHeaders map[string][]string
	Cancel       context.CancelFunc
}

func RandStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = lBytes[rand.Intn(len(lBytes))]
	}
	return strings.ToLower(string(b))
}

func SetupDefaultSuite() *SuiteSetup {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	ss := &SuiteSetup{
		Context:      ctx,
		Client:       APIClient(),
		AppName:      "fnintegrationtestapp" + RandStringBytes(10),
		RoutePath:    "/fnintegrationtestroute" + RandStringBytes(10),
		Image:        "fnproject/hello",
		Format:       "default",
		RouteType:    "async",
		RouteConfig:  map[string]string{},
		RouteHeaders: map[string][]string{},
		Cancel:       cancel,
		Memory:       uint64(256),
	}

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

	return ss
}

func Cleanup() {
	ctx := context.Background()
	c := APIClient()
	approutesLock.Lock()
	defer approutesLock.Unlock()
	for appName, rs := range appsandroutes {
		for _, routePath := range rs {
			DeleteRouteNoAssert(ctx, c, appName, routePath)
		}
		DeleteAppNoT(ctx, c, appName)
	}
	appsandroutes = make(map[string][]string)
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
	fmt.Println("call response status code: ", resp.Status)
	io.Copy(output, resp.Body)

	return resp, nil
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func MyCaller() string {
	fpcs := make([]uintptr, 1)
	n := runtime.Callers(3, fpcs)
	if n == 0 {
		return "n/a"
	}
	fun := runtime.FuncForPC(fpcs[0] - 1)
	if fun == nil {
		return "n/a"
	}
	f, l := fun.FileLine(fpcs[0] - 1)
	return fmt.Sprintf("%s:%d", f, l)
}

func CallAsync(t *testing.T, u url.URL, content io.Reader) string {
	output := &bytes.Buffer{}
	_, err := CallFN(u.String(), content, output, "POST", []string{})
	if err != nil {
		t.Errorf("Got unexpected error: %v", err)
	}

	expectedOutput := "call_id"
	if !strings.Contains(output.String(), expectedOutput) {
		t.Errorf("Assertion error.\n\tExpected: %v\n\tActual: %v", expectedOutput, output.String())
	}

	type CallID struct {
		CallID string `json:"call_id"`
	}

	callID := &CallID{}
	json.NewDecoder(output).Decode(callID)

	if callID.CallID == "" {
		t.Errorf("`call_id` not suppose to be empty string")
	}
	t.Logf("Async execution call ID: %v", callID.CallID)
	return callID.CallID
}
