# Oracle Functions LoadBalancer

## Loadbalancing several Oracle Functions
You can run multiple Oracle Functions instances and balance the load amongst them using `fnlb` as follows:

```sh
fnlb --listen <address-for-incoming> --nodes <node1>,<node2>,<node3>
```

And redirect all traffic to the load balancer.

**NOTE: For the load balancer to work all function nodes need to be sharing the same DB.**

## Running with docker

```sh
make docker-build
make docker-run
curl -X PUT -d '{"node":"127.0.0.1:8080"}' localhost:8081/1/lb/nodes
```

`127.0.0.1:8080` should be the address of a functions server. The lb will health
check this and log if it cannot reach that node. By default, docker-run runs
with --net=host and should work out of the box. Any number of functions
servers may be added to the load balancer.

To make functions requests against the lb:

```sh
API_URL=http://localhost:8081 fn call my/function
```

## TODO More docs to be added for complex installs..
