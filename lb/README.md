# IronFunctions LoadBalancer

## Loadbalancing several IronFunctions
You can run multiple IronFunctions instances and balance the load amongst them using `fnlb` as follows:

```sh
fnlb --listen <address-for-incoming> --nodes <node1>,<node2>,<node3>
```

And redirect all traffic to the load balancer.

**NOTE: For the load balancer to work all function nodes need to be sharing the same DB.**