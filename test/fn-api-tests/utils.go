package tests

import (
	"context"
	"strings"
	"sync"
	"time"

	"gitlab-odx.oracle.com/odx/functions/api/server"

	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"

	"github.com/funcy/functions_go/client"
	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	"github.com/spf13/viper"
)

const lBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

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
	return client.New(transport, strfmt.Default)
}

var (
	getServer sync.Once
	cancel2   context.CancelFunc
	s         *server.Server
)

func getServerWithCancel() (*server.Server, context.CancelFunc) {
	getServer.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())

		viper.Set(server.EnvPort, "8080")
		viper.Set(server.EnvAPIURL, "http://localhost:8080")
		viper.Set(server.EnvLogLevel, "fatal")
		timeString := time.Now().Format("2006_01_02_15_04_05")
		db_url := os.Getenv("DB_URL")
		tmpDir := os.TempDir()
		tmpMq := fmt.Sprintf("%s/fn_integration_test_%s_worker_mq.db", tmpDir, timeString)
		tmpDb := fmt.Sprintf("%s/fn_integration_test_%s_fn.db", tmpDir, timeString)
		viper.Set(server.EnvMQURL, fmt.Sprintf("bolt://%s", tmpMq))
		if db_url == "" {
			db_url = fmt.Sprintf("sqlite3://%s", tmpDb)
		}
		viper.Set(server.EnvDBURL, db_url)

		s = server.NewFromEnv(ctx)

		go s.Start(ctx)
		started := false
		time.AfterFunc(time.Second*10, func() {
			if !started {
				panic("Failed to start server.")
			}
		})
		log.Println(server.EnvAPIURL)
		_, err := http.Get(viper.GetString(server.EnvAPIURL) + "/version")
		for err != nil {
			_, err = http.Get(viper.GetString(server.EnvAPIURL) + "/version")
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

func RandStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = lBytes[rand.Intn(len(lBytes))]
	}
	return strings.ToLower(string(b))
}

func SetupDefaultSuite() *SuiteSetup {
	ss := &SuiteSetup{
		Context:      context.Background(),
		Client:       APIClient(),
		AppName:      RandStringBytes(10),
		RoutePath:    "/" + RandStringBytes(10),
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
