# fnlb-test-harness
Test harness that exercises the fnlb load balancer in order to verify that it works properly.
## How it works
This is a test harness that makes calls to an IronFunctions route through the fnlb load balancer, which routes traffic to multiple IronFunctions nodes.
The test harness keeps track of which node each request was routed to so we can assess how the requests are being distributed across the nodes.  The functionality
of fnlb is to normally route traffic to the same small number of nodes so that efficiences can be achieved and to support reuse of hot functions.
### Primes function
The test harness utilizes the "primes" function, which calculates prime numbers as an excuse for consuming CPU resources.  The function is invoked as follows:
```
curl http://host:8080/r/primesapp/primes?max=1000000&loops=1
```
where:
- *max*: calculate all primes <= max (increasing max will increase memory usage, due to the Sieve of Eratosthenes algorithm)
- *loops*: number of times to calculate the primes (repeating the count consumes additional CPU without consuming additional memory)

## How to use it
The test harness requires running one or more IronFunctions nodes and one instance of fnlb.  The list of nodes must be provided both to fnlb and to the test harness
because the test harness must call each node directly one time in order to discover the node's container id.

After it has run, examine the results to see how the requests were distributed across the nodes.
### How to run it locally
Each of the IronFunctions nodes needs to connect to the same database.

STEP 1: Create a route for the primes function.  Example:
```
fn apps create primesapp
fn routes create primesapp /primes jconning/primes:0.0.1
```
STEP 2: Run five IronFunctions nodes locally.  Example (runs five nodes in the background using Docker):
```
sudo docker run -d -it --name functions-8082 --privileged -v ${HOME}/data-8082:/app/data -p 8082:8080 -e "DB_URL=postgres://dbUser:dbPassword@dbHost:5432/dbName" iron/functions
sudo docker run -d -it --name functions-8083 --privileged -v ${HOME}/data-8083:/app/data -p 8083:8080 -e "DB_URL=postgres://dbUser:dbPassword@dbHost:5432/dbName" iron/functions
sudo docker run -d -it --name functions-8084 --privileged -v ${HOME}/data-8084:/app/data -p 8084:8080 -e "DB_URL=postgres://dbUser:dbPassword@dbHost:5432/dbName" iron/functions
sudo docker run -d -it --name functions-8085 --privileged -v ${HOME}/data-8085:/app/data -p 8085:8080 -e "DB_URL=postgres://dbUser:dbPassword@dbHost:5432/dbName" iron/functions
sudo docker run -d -it --name functions-8086 --privileged -v ${HOME}/data-8086:/app/data -p 8086:8080 -e "DB_URL=postgres://dbUser:dbPassword@dbHost:5432/dbName" iron/functions
```
STEP 3: Run fnlb locally.  Example (runs fnlb on the default port 8081):
```
fnlb -nodes localhost:8082,localhost:8083,localhost:8084,localhost:8085,localhost:8086
```
STEP 4: Run the test harness.  Note that the 'nodes' parameter should be the same that was used with fnlb.  Example:
```
cd functions/test/fnlb-test-harness
go run main.go -nodes localhost:8082,localhost:8083,localhost:8084,localhost:8085,localhost:8086 -calls 10 -v
```
STEP 5: Examine the output to determine how many times fnlb called each node.  Assess whether it is working properly.

### Usage
go run main.go -help

<i>Command line parameters:</i>
- *-calls*: number of times to call the route (default 100)
- *-lb*: host and port of load balancer (default "localhost:8081")
- *-loops*: number of times to execute the primes calculation (ex: '-loops 2' means run the primes calculation twice) (default 1)
- *-max*: maximum number to search for primes (higher number consumes more memory) (default 1000000)
- *-nodes*: comma-delimited list of nodes (host:port) balanced by the load balancer (needed to discover container id of each) (default "localhost:8080")
- *-route*: path representing the route to the primes function (default "/r/primesapp/primes")
- *-v*: flag indicating verbose output

### Examples: quick vs long running

**Quick function:**: calculate primes up to 1000
```
go run main.go -nodes localhost:8082,localhost:8083,localhost:8084,localhost:8085,localhost:8086 -max 1000 -v
```
where *-max* is default of 1M, *-calls* is default of 100, *-route* is default of "/r/primesapp/primes", *-lb* is default localhost:8081

**Normal function**: calculate primes up to 1M
```
go run main.go -nodes localhost:8082,localhost:8083,localhost:8084,localhost:8085,localhost:8086 -v
```
where *-max* is default of 1M, *-calls* is default of 100, *-route* is default of "/r/primesapp/primes", *-lb* is default localhost:8081

**Longer running function**: calculate primes up to 1M and perform the calculation ten times
```
go run main.go -nodes localhost:8082,localhost:8083,localhost:8084,localhost:8085,localhost:8086 -loops 10 -v
```
where *-max* is default of 1M, *-calls* is default of 100, *-route* is default of "/r/primesapp/primes", *-lb* is default localhost:8081

**1000 calls to the route**: send 1000 requests through the load balancer
```
go run main.go -nodes localhost:8082,localhost:8083,localhost:8084,localhost:8085,localhost:8086 -calls 1000 -v
```
where *-max* is default of 1M, *-calls* is default of 100, *-route* is default of "/r/primesapp/primes", *-lb* is default localhost:8081

## Planned Enhancements
- Create 1000 routes and distribute calls amongst them.
- Use concurrent programming to enable the test harness to call multiple routes at the same time.
