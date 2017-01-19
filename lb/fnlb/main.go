package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/iron-io/functions/lb"
)

var (
	fnodes  string
	flisten string
)

func init() {
	flag.StringVar(&fnodes, "nodes", "127.0.0.1:8080", "comma separated list of IronFunction nodes")
	flag.StringVar(&flisten, "listen", "0.0.0.0:8081", "listening port for incoming connections")
	flag.Parse()
}

func main() {
	nodes := strings.Split(fnodes, ",")
	p := lb.ConsistentHashReverseProxy(context.Background(), nodes)
	fmt.Println("forwarding calls to", nodes)
	fmt.Println("listening to", flisten)
	if err := http.ListenAndServe(flisten, p); err != nil {
		fmt.Fprintln(os.Stderr, "could not start server. error:", err)
		os.Exit(1)
	}
}
