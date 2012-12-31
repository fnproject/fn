/*

For keeping a minimum running, perhaps when doing a routing table update, if destination hosts are all
 expired or about to expire we start more. 

*/

package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

var routingTable = map[string]Route{}

type Route struct {
	// TODO: Change destinations to a simple cache so it can expire entries after 55 minutes (the one we use in common?)
	Destinations []string
}

// for adding new hosts
type Route2 struct {
	Host string `json:"host"`
	Dest string `json:"dest"`
}

func main() {
	// verbose := flag.Bool("v", true, "should every proxy request be logged to stdout")
	// flag.Parse()

	r := mux.NewRouter()
	s := r.Headers("Iron-Router", "").Subrouter()
	s.HandleFunc("/", AddWorker)
	r.HandleFunc("/addworker", AddWorker)

	r.HandleFunc("/", ProxyFunc)

	http.Handle("/", r)
	port := 80
	fmt.Println("listening and serving on port", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", port), nil))
}

func ProxyFunc(w http.ResponseWriter, req *http.Request) {
	fmt.Println("HOST:", req.Host)
	host := strings.Split(req.Host, ":")[0]
	route := routingTable[host]
	// choose random dest
	if len(route.Destinations) == 0 {
		fmt.Fprintln(w, "No matching routes!")
		return
	}
	destUrls := route.Destinations[rand.Intn(len(route.Destinations))]
	// todo: should check if http:// already exists.
	destUrls = "http://" + destUrls
	destUrl, err := url.Parse(destUrls)
	if err != nil {
		fmt.Println("error!", err)
		panic(err)
	}
	fmt.Println("proxying to", destUrl)
	proxy := httputil.NewSingleHostReverseProxy(destUrl)
	proxy.ServeHTTP(w, req)
	// todo: how to handle destination failures. I got this in log output when testing a bad endpoint:
	// 2012/12/26 23:22:08 http: proxy error: dial tcp 127.0.0.1:8082: connection refused
}

func AddWorker(w http.ResponseWriter, req *http.Request) {
	log.Println("AddWorker called!")
	r2 := Route2{}
	decoder := json.NewDecoder(req.Body)
	decoder.Decode(&r2)
	// todo: do we need to close body?
	fmt.Println("DECODED:", r2)

	// todo: routing table should be in mongo (or IronCache?) so all routers can update/read from it.
	route := routingTable[r2.Host]
	fmt.Println("ROUTE:", route)
	route.Destinations = append(route.Destinations, r2.Dest)
	fmt.Println("ROUTE:", route)
	routingTable[r2.Host] = route
	fmt.Println("New routing table:", routingTable)
	fmt.Fprintln(w, "Worker added")
}
