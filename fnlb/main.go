package main

import (
	"context"
	"crypto/tls"
	"flag"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/fnproject/fn/fnlb/lb"
	"github.com/sirupsen/logrus"
)

const VERSION = "0.0.118"

func main() {
	// XXX (reed): normalize
	fnodes := flag.String("nodes", "", "comma separated list of functions nodes")
	minAPIVersion := flag.String("min-api-version", "0.0.85", "minimal node API to accept")

	var conf lb.Config
	flag.StringVar(&conf.DBurl, "db", "sqlite3://:memory:", "backend to store nodes, default to in memory")
	flag.StringVar(&conf.Listen, "listen", ":8081", "port to run on")
	flag.IntVar(&conf.HealthcheckInterval, "hc-interval", 3, "how often to check f(x) nodes, in seconds")
	flag.StringVar(&conf.HealthcheckEndpoint, "hc-path", "/version", "endpoint to determine node health")
	flag.IntVar(&conf.HealthcheckUnhealthy, "hc-unhealthy", 2, "threshold of failed checks to declare node unhealthy")
	flag.IntVar(&conf.HealthcheckTimeout, "hc-timeout", 5, "timeout of healthcheck endpoint, in seconds")
	flag.StringVar(&conf.ZipkinURL, "zipkin", "", "zipkin endpoint to send traces")
	flag.Parse()

	conf.MinAPIVersion = semver.New(*minAPIVersion)

	if len(*fnodes) > 0 {
		// starting w/o nodes is fine too
		conf.Nodes = strings.Split(*fnodes, ",")
	}

	conf.Transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 120 * time.Second,
		}).Dial,
		MaxIdleConnsPerHost: 512,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig: &tls.Config{
			ClientSessionCache: tls.NewLRUClientSessionCache(4096),
		},
	}

	g, err := lb.NewAllGrouper(conf)
	if err != nil {
		logrus.WithError(err).Fatal("error setting up grouper")
	}

	r := lb.NewConsistentRouter(conf)
	k := func(r *http.Request) (string, error) {
		return r.URL.Path, nil
	}

	h := lb.NewProxy(k, g, r, conf)
	h = g.Wrap(h) // add/del/list endpoints
	h = r.Wrap(h) // stats / dash endpoint

	err = serve(conf.Listen, h)
	if err != nil {
		logrus.WithError(err).Fatal("server error")
	}
}

func serve(addr string, handler http.Handler) error {
	server := &http.Server{Addr: addr, Handler: handler}

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGQUIT, syscall.SIGINT)
	go func() {
		for sig := range ch {
			logrus.WithFields(logrus.Fields{"signal": sig}).Info("received signal")
			server.Shutdown(context.Background()) // safe shutdown
			return
		}
	}()
	return server.ListenAndServe()
}
