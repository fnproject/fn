# HOWTO run Oracle Functions in Kubernetes at AWS

*Prerequisite 1: it assumes you have a working Kubernetes, and a locally configured kubectl.*

*Prerequisite 2: It assumes you are using Kubernetes 1.4 or newer.*


## Quickstart

### Steps

1. Start Oracle Functions in the Kubernetes cluster:
```ShellSession
$ cd docs/
$ kubectl create -f kubernetes-quick
```

2. Once the daemon is started, check where it is listening for connections:

```ShellSession
# kubectl describe svc functions

Name:			functions
Namespace:		default
Labels:			app=functions
Selector:		app=functions
Type:			LoadBalancer
IP:			10.0.116.122
LoadBalancer Ingress:	a23122e39900111e681ba0e29b70bb46-630391493.us-east-1.elb.amazonaws.com
Port:			<unset>	8080/TCP
NodePort:		<unset>	30802/TCP
Endpoints:		10.244.1.12:8080
Session Affinity:	None
Events:
  FirstSeen	LastSeen	Count	From			SubobjectPath	Type		Reason			Message
  ---------	--------	-----	----			-------------	--------	------			-------
  22m		22m		1	{service-controller }			Normal		CreatingLoadBalancer	Creating load balancer
  22m		22m		1	{service-controller }			Normal		CreatedLoadBalancer	Created load balancer

```

Note `a23122e39900111e681ba0e29b70bb46-630391493.us-east-1.elb.amazonaws.com` in `LoadBalancer Ingress` line, this is where the service is listening.

3. Test the cluster:

```ShellSession
$ export FUNCTIONS=$(kubectl get -o json svc functions | jq -r '.status.loadBalancer.ingress[0].hostname'):8080

$ curl -H "Content-Type: application/json" -X POST -d '{ "app": { "name":"myapp" } }' http://$FUNCTIONS/v1/apps
{"message":"App successfully created","app":{"name":"myapp","config":null}}

$ curl -H "Content-Type: application/json" -X POST -d '{ "route": { "type": "sync", "path":"/hello-sync", "image":"treeder/hello" } }' http://$FUNCTIONS/v1/apps/myapp/routes
{"message":"Route successfully created","route":{"app_name":"myapp","path":"/hello-sync","image":"treeder/hello","memory":128,"type":"sync","config":null}}

$ curl -H "Content-Type: application/json" -X POST -d '{ "name":"Johnny" }' http://$FUNCTIONS/r/myapp/hello-sync
Hello Johnny!
```

## Production

### Steps

1. Start Oracle Functions and its dependencies:
```ShellSession
$ cd docs/
$ kubectl create -f kubernetes-production
```

*Optionally, you might have both Redis and PostgreSQL started somewhere else, in this case, remember to update kubernetes-production/functions-config.yaml with the appropriate configuration.*

2. Once the daemon is started, check where it is listening for connections:

```ShellSession
# kubectl describe svc functions

Name:			functions
Namespace:		default
Labels:			app=functions
Selector:		app=functions
Type:			LoadBalancer
IP:			10.0.116.122
LoadBalancer Ingress:	a23122e39900111e681ba0e29b70bb46-630391493.us-east-1.elb.amazonaws.com
Port:			<unset>	8080/TCP
NodePort:		<unset>	30802/TCP
Endpoints:		10.244.1.12:8080
Session Affinity:	None
Events:
  FirstSeen	LastSeen	Count	From			SubobjectPath	Type		Reason			Message
  ---------	--------	-----	----			-------------	--------	------			-------
  22m		22m		1	{service-controller }			Normal		CreatingLoadBalancer	Creating load balancer
  22m		22m		1	{service-controller }			Normal		CreatedLoadBalancer	Created load balancer

```

Note `a23122e39900111e681ba0e29b70bb46-630391493.us-east-1.elb.amazonaws.com` in `LoadBalancer Ingress` line, this is where the service is listening.

3. Test the cluster:

```ShellSession
$ export FUNCTIONS=$(kubectl get -o json svc functions | jq -r '.status.loadBalancer.ingress[0].hostname'):8080

$ curl -H "Content-Type: application/json" -X POST -d '{ "app": { "name":"myapp" } }' http://$FUNCTIONS/v1/apps
{"message":"App successfully created","app":{"name":"myapp","config":null}}

$ curl -H "Content-Type: application/json" -X POST -d '{ "route": { "type": "sync", "path":"/hello-sync", "image":"treeder/hello" } }' http://$FUNCTIONS/v1/apps/myapp/routes
{"message":"Route successfully created","route":{"app_name":"myapp","path":"/hello-sync","image":"treeder/hello","memory":128,"type":"sync","config":null}}

$ curl -H "Content-Type: application/json" -X POST -d '{ "name":"Johnny" }' http://$FUNCTIONS/r/myapp/hello-sync
Hello Johnny!
```