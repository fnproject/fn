package main

import (
	"fmt"
	"net/http"
	"io/ioutil"
	"encoding/json"
	"time"
	"flag"
	"log"
	"strings"
)

type execution struct {
	DurationSeconds float64
	Hostname string
	node string
	body string
	responseSeconds float64
}

var (
	lbHostPort, nodesStr, route string
	numExecutions, maxPrime, numLoops int
	nodes []string
	nodesByContainerId map[string]string = make(map[string]string)
	verbose bool
)

func init() {
	flag.StringVar(&lbHostPort, "lb", "localhost:8081", "host and port of load balancer")
	flag.StringVar(&nodesStr, "nodes", "localhost:8080", "comma-delimited list of nodes (host:port) balanced by the load balancer (needed to discover container id of each)")
	flag.StringVar(&route, "route", "/r/primesapp/primes", "path representing the route to the primes function")
	flag.IntVar(&numExecutions, "calls", 100, "number of times to call the route")
	flag.IntVar(&maxPrime, "max", 1000000, "maximum number to search for primes (higher number consumes more memory)")
	flag.IntVar(&numLoops, "loops", 1, "number of times to execute the primes calculation (ex: 'loops=2' means run the primes calculation twice)")
	flag.BoolVar(&verbose, "v", false, "true for more verbose output")
	flag.Parse()

	if maxPrime < 3 {
		log.Fatal("-max must be 3 or greater")
	}
	if numLoops < 1 {
		log.Fatal("-loops must be 1 or greater")
	}

	nodes = strings.Split(nodesStr, ",")
}

func executeFunction(hostPort, path string, max, loops int) (execution, error) {
	var e execution

	start := time.Now()
	resp, err := http.Get(fmt.Sprintf("http://%s%s?max=%d&loops=%d", hostPort, path, max, loops))
	e.responseSeconds = time.Since(start).Seconds()
	if err != nil {
		return e, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return e, fmt.Errorf("function returned status code: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return e, err
	}

	err = json.Unmarshal(body, &e)
	if err != nil {
		e.body = string(body) // set the body in the execution so that it is available for logging
		return e, err
	}
	e.node = nodesByContainerId[e.Hostname]

	return e, nil
}

func invokeLoadBalancer(hostPort, path string, numExecutions, max, loops int) {
	executionsByNode := make(map[string][]execution)
	fmt.Printf("All primes will be calculated up to %d, a total of %d time(s)\n", maxPrime, numLoops)
	fmt.Printf("Calling route %s %d times (through the load balancer)...\n", route, numExecutions)

	for i := 0; i < numExecutions; i++ {
		e, err := executeFunction(hostPort, path, max, loops)
		if err == nil {
			if ex, ok := executionsByNode[e.node]; ok {
				executionsByNode[e.node] = append(ex, e)
			} else {
				// Create a slice to contain the list of executions for this host
				executionsByNode[e.node] = []execution{e}
			}
			if verbose {
				fmt.Printf("  %s in-function duration: %fsec, response time: %fsec\n", e.node, e.DurationSeconds, e.responseSeconds)
			}
		} else {
			fmt.Printf("  Ignoring failed execution on node %s: %v\n", e.node, err)
			fmt.Printf("    JSON: %s\n", e.body)
		}
	}

	fmt.Println("Results (executions per node):")
	for node, ex := range executionsByNode {
		fmt.Printf("  %s %d\n", node, len(ex))
	}
}

func discoverContainerIds() {
	// Discover the Docker hostname of each node; create a mapping of hostnames to host/port.
	// This is needed because IronFunctions doesn't make the host/port available to the function (as of Mar 2017).
	fmt.Println("Discovering container ids for every node (use Docker's HOSTNAME env var as a container id)...")
	for _, s := range nodes {
		if e, err := executeFunction(s, route, 100, 1); err == nil {
			nodesByContainerId[e.Hostname] = s
			fmt.Printf("  %s %s\n", s, e.Hostname)
		} else {
			fmt.Printf("  Ignoring host %s which returned error: %v\n", s, err)
			fmt.Printf("    JSON: %s\n", e.body)
		}
	}
}

func main() {
	discoverContainerIds()
	invokeLoadBalancer(lbHostPort, route, numExecutions, maxPrime, numLoops)
}

