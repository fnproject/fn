# How to run Fn on Kubernetes

*Prerequisite 1: working Kubernetes cluster (v1.7+), and a locally configured kubectl.*

## Quickstart

### Steps

1. Deploy Fn to the Kubernetes cluster:

```bash
$ cd docs/operating/kubernetes/
$ kubectl create -f fn-service.yaml
```

2. Once the Pods have started, check the service for the load balancer IP:

```bash
$ kubectl -n fn get svc --watch
NAME                                CLUSTER-IP      EXTERNAL-IP     PORT(S)                                                       AGE
fn-mysql-master                     10.96.57.185    <none>          3306/TCP                                                      10m
fn-redis-master                     10.96.127.51    <none>          6379/TCP                                                      10m
fn-service                          10.96.245.95    <pending>       8080:30768/TCP,80:31921/TCP                                   10m
kubernetes                          10.96.0.1       <none>          443/TCP                                                       15d
```

Note that `fn-service` is initially pending on allocating an external IP. The `kubectl get svc --watch` command  will update this once an IP has been assigned.

3. Test the cluster:

If you are using a Kubernetes setup that can expose a public load balancer, run:

```bash
$ export FUNCTIONS=$(kubectl -n fn get -o json svc fn-service | jq -r '.status.loadBalancer.ingress[0].ip'):8080
```

If you are using a Kubernetes setup like minikube, run

```bash
$ export API_URL=$(minikube -n fn service fn-service --url)
```

Now, test by creating a function via curl:

```bash
$ curl -H "Content-Type: application/json" -X POST -d '{ "app": { "name":"myapp" } }' http://$API_URL/v1/apps
{"message":"App successfully created","app":{"name":"myapp","config":null}}

$ curl -H "Content-Type: application/json" -X POST -d '{ "route": { "type": "sync", "path":"/hello-sync", "image":"fnproject/hello" } }' http://$API_URL/v1/apps/myapp/routes
{"message":"Route successfully created","route":{"app_name":"myapp","path":"/hello-sync","image":"fnproject/hello","memory":128,"headers":{},"type":"sync","format":"default","timeout":30,"idle_timeout":30,"config":{}}}

$ curl -H "Content-Type: application/json" -X POST -d '{ "name":"Johnny" }' http://$API_URL/r/myapp/hello-sync
Hello Johnny!
```

You can also use the [Fn CLI](https://github.com/fnproject/cli):

```bash
$ export API_URL=http://192.168.99.100:30966
$ fn apps list
myapp
$ fn routes list myapp
path            image           endpoint
/hello-sync     fnproject/hello 192.168.99.100:30966/r/myapp/hello-sync
```
