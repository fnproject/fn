# Fn Load Balancer

The Fn Load Balancer (Fn LB) allows operators to deploy clusters of Fn servers and route traffic to them intelligently. Most importantly, it will route traffic to nodes where hot functions are running to ensure optimal performance, as well as distribute load if traffic to a specific function increases. It also gathers information about the entire cluster which you can use to know when to scale out (add more Fn servers) or in (decrease Fn servers).

## Load balancing several Fn servers
You can run multiple Fn instances and balance the load amongst them using `fnlb` as follows:

```sh
fnlb --listen <address-for-incoming> --nodes <node1>,<node2>,<node3>
```

And redirect all traffic to the load balancer.

**NOTE: For the load balancer to be of use, all function nodes need to be sharing the same DB.**

## Running with docker

To build a docker image for `fnlb` just run (in `fnlb/`):

```
make docker-build
```

To start the `fnlb` proxy with the addresses of functions nodes in a docker
container:

```sh
docker run -d --name fnlb -p 8081:8081 fnproject/fnlb:latest --nodes <node1>,<node2>
```

If running locally with functions servers in docker, running with docker links
can make things easier (can use local addresses). for example:

```sh
docker run -d --name fn-8080 --privileged -p 8080:8080 fnproject/functions:latest
docker run -d --name fnlb --link fn-8080 -p 8081:8081 fnproject/fnlb:latest --nodes 127.0.0.1:8080
```

## Operating / usage

To make functions requests against the lb with the cli:

```sh
API_URL=http://<fnlb_address> fn call my/function
```

To add a functions node later:

```sh
curl -sSL -X PUT -d '{"node":"<node>"}' <fnlb_address>/1/lb/nodes
```

`<node>` should be the address of a functions server. The lb will health check
this and log if it cannot reach that node as well as stop sending requests to
that node until it begins passing health checks again. Any number of functions
servers may be added to the load balancer.

To permanently remove a functions node:

```sh
curl -sSL -X DELETE -d '{"node":"<node>"}' <fnlb_address>/1/lb/nodes
```

To list functions nodes and their state:

```sh
curl -sSL -X GET <fnlb_address>/1/lb/nodes
```
