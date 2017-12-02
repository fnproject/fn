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

const VERSION = "0.0.177"

func main() {
	// XXX (reed): normalize
	fnodes := flag.String("nodes", "", "comma separated list of functions nodes")
	minAPIVersion := flag.String("min-api-version", "0.0.144", "minimal node API to accept")

	var conf lb.Config
	flag.StringVar(&conf.DBurl, "db", "sqlite3://:memory:", "backend to store nodes, default to in memory")
	flag.StringVar(&conf.Listen, "listen", ":8081", "port to run on")
	flag.StringVar(&conf.MgmtListen, "mgmt-listen", ":8081", "management port to run on")
	flag.IntVar(&conf.ShutdownTimeout, "shutdown-timeout", 0, "graceful shutdown timeout")
	flag.IntVar(&conf.HealthcheckInterval, "hc-interval", 3, "how often to check f(x) nodes, in seconds")
	flag.StringVar(&conf.HealthcheckEndpoint, "hc-path", "/version", "endpoint to determine node health")
	flag.IntVar(&conf.HealthcheckUnhealthy, "hc-unhealthy", 2, "threshold of failed checks to declare node unhealthy")
	flag.IntVar(&conf.HealthcheckHealthy, "hc-healthy", 1, "threshold of success checks to declare node healthy")
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

	db, err := lb.NewDB(conf)
	if err != nil {
		logrus.WithError(err).Fatal("error setting up database")
	}

	g, err := lb.NewAllGrouper(conf, db)
	if err != nil {
		logrus.WithError(err).Fatal("error setting up grouper")
	}

	r := lb.NewConsistentRouter(conf)
	k := func(r *http.Request) (string, error) {
		return r.URL.Path, nil
	}

	servers := make([]*http.Server, 0, 1)
	handler := lb.NewProxy(k, g, r, conf)

	// a separate mgmt listener is requested? then let's create a LB traffic only server
	if conf.Listen != conf.MgmtListen {
		servers = append(servers, &http.Server{Addr: conf.Listen, Handler: handler})
		handler = lb.NullHandler()
	}

	// add mgmt endpoints to the handler
	handler = g.Wrap(handler) // add/del/list endpoints
	handler = r.Wrap(handler) // stats / dash endpoint

	servers = append(servers, &http.Server{Addr: conf.MgmtListen, Handler: handler})
	serve(servers, &conf)
}

func serve(servers []*http.Server, conf *lb.Config) {

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGQUIT, syscall.SIGINT)

	for i := 0; i < len(servers); i++ {
		go func(idx int) {
			err := servers[idx].ListenAndServe()
			if err != nil && err != http.ErrServerClosed {
				logrus.WithFields(logrus.Fields{"server_id": idx}).WithError(err).Fatal("server error")
			} else {
				logrus.WithFields(logrus.Fields{"server_id": idx}).Info("server stopped")
			}
		}(i)
	}

	sig := <-ch
	logrus.WithFields(logrus.Fields{"signal": sig}).Info("received signal")

	for i := 0; i < len(servers); i++ {

		ctx := context.Background()

		if conf.ShutdownTimeout > 0 {
			tmpCtx, cancel := context.WithTimeout(context.Background(), time.Duration(conf.ShutdownTimeout)*time.Second)
			ctx = tmpCtx
			defer cancel()
		}

		err := servers[i].Shutdown(ctx) // safe shutdown
		if err != nil {
			logrus.WithFields(logrus.Fields{"server_id": i}).WithError(err).Fatal("server shutdown error")
		} else {
			logrus.WithFields(logrus.Fields{"server_id": i}).Info("server shutdown")
		}
	}
}
