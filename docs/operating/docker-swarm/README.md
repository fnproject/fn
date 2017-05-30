# Docker Swarm and Oracle Functions

How to run Oracle Functions as a scheduler on top of Docker Standalone Swarm cluster.

## Quick installation

*Prerequisite 1: Make sure you have a working Docker 1.12+ Standalone Swarm cluster in place, you can build one by following the instructions at [Docker's website](https://docs.docker.com/swarm/).*

*Prerequisite 2: It assumes that your running environment is already configured to use Swarm's master scheduler.*

This is a step-by-step procedure to execute Oracle Functions on top of Docker Swarm cluster. It works by having Oracle Functions daemon started through Swarm's master, and there enqueueing tasks through Swarm API.

### Steps

1. Start Oracle Functions in the Swarm Master. It expects all basic Docker environment variables to be present (DOCKER_TLS_VERIFY, DOCKER_HOST, DOCKER_CERT_PATH, DOCKER_MACHINE_NAME). The important part is that the working Swarm master environment must be passed to Functions daemon:
```ShellSession
$ docker login # if you plan to use private images
$ docker volume create --name functions-datafiles
$ docker run -d --name functions \
        -p 8080:8080 \
        -e DOCKER_TLS_VERIFY \
        -e DOCKER_HOST \
        -e DOCKER_CERT_PATH="/docker-cert" \
        -e DOCKER_MACHINE_NAME \
        -v $DOCKER_CERT_PATH:/docker-cert \
        -v functions-datafiles:/app/data \
        treeder/functions
```

2. Once the daemon is started, check where it is listening for connections:

```ShellSession
# docker info
CONTAINER ID        IMAGE                COMMAND                  CREATED             STATUS              PORTS                                     NAMES
5a0846e6a025        treeder/functions       "/usr/local/bin/entry"   59 seconds ago      Up 58 seconds       2375/tcp, 10.0.0.1:8080->8080/tcp   swarm-agent-00/functions
````

Note `10.0.0.1:8080` in `PORTS` column, this is where the service is listening. Oracle Functions will use Docker Swarm scheduler to deliver tasks to all nodes present in the cluster.

3. Test the cluster:

```ShellSession
$ export FUNCTIONS=$(docker port functions | cut -d ' ' -f3)

$ curl -H "Content-Type: application/json" -X POST -d '{ "app": { "name":"myapp" } }' http://$FUNCTIONS/v1/apps
{"message":"App successfully created","app":{"name":"myapp","config":null}}

$ curl -H "Content-Type: application/json" -X POST -d '{ "route": { "type": "sync", "path":"/hello-sync", "image":"funcy/hello" } }' http://$FUNCTIONS/v1/apps/myapp/routes
{"message":"Route successfully created","route":{"app_name":"myapp","path":"/hello-sync","image":"funcy/hello","memory":128,"type":"sync","config":null}}

$ curl -H "Content-Type: application/json" -X POST -d '{ "name":"Johnny" }' http://$FUNCTIONS/r/myapp/hello-sync
Hello Johnny!
```

## Production installation

*Prerequisite 1: Make sure you have a working Docker Standalone Swarm cluster with multi-node network mode in place, you can build one by following the instructions at [Docker's website](https://docs.docker.com/swarm/). The instructions to build a multi-host network can be found at [Docker's engine manual](https://docs.docker.com/engine/userguide/networking/get-started-overlay/#overlay-networking-with-an-external-key-value-store).*

*Prerequisite 2: It assumes that your running environment is already configured to use Swarm's master scheduler.*

This is a step-by-step procedure to execute Oracle Functions on top of Docker Swarm cluster. It works by having Oracle Functions daemon started through Swarm's master, however the tasks are executed on each host locally. In production, database and message queue must be external to Oracle Functions execution, this guarantees robustness of the service against failures.

We strongly recommend you deploy your own HA Redis and PostgreSQL clusters. Otherwise, you can follow the instructions below and have them set in single nodes.

### Groundwork

Although we're assuming you already have your Docker Swarm installed and configured, these `docker-machine` calls are instructive regarding some configuration details:
```bash
#!/bin/bash

# Note how every host points to an external etcd both for swarm discovery (--swarm-discovery) as much as network configuration (--engine-opt=cluster-store=)
docker-machine create -d virtualbox --swarm --swarm-master --swarm-discovery etcd://$ETCD_HOST:2379/swarm --engine-opt="cluster-store=etcd://$ETCD_HOST:2379/network" --engine-opt="cluster-advertise=eth1:2376" swarm-manager;

# Set aside one host for DB activities
docker-machine create -d virtualbox --engine-label use=db --swarm --swarm-discovery etcd://$ETCD_HOST:2379/swarm --engine-opt="cluster-store=etcd://$ETCD_HOST:2379/network" --engine-opt="cluster-advertise=eth1:2376" swarm-db;

# The rest is a horizontally scalable set of hosts for Oracle Functions
docker-machine create -d virtualbox --engine-label use=worker --swarm --swarm-discovery etcd://$ETCD_HOST:2379/swarm --engine-opt="cluster-store=etcd://$ETCD_HOST:2379/network" --engine-opt="cluster-advertise=eth1:2376" swarm-worker-00;
docker-machine create -d virtualbox --engine-label use=worker --swarm --swarm-discovery etcd://$ETCD_HOST:2379/swarm --engine-opt="cluster-store=etcd://$ETCD_HOST:2379/network" --engine-opt="cluster-advertise=eth1:2376" swarm-worker-01
```

### Steps

If you using externally deployed Redis and PostgreSQL cluster, you may skip to step 4.

1. Build a multi-host network for Oracle Functions:
```ShellSession
$ docker network create --driver overlay --subnet=10.0.9.0/24 functions-network
````

2. Setup Redis as message queue service:
```ShellSession
$ docker create -e constraint:use==db --network=functions-network -v /data --name redis-data redis /bin/true;
$ docker run -d -e constraint:use==db --network=functions-network --volumes-from redis-data --name functions-redis redis;
````

3. Setup PostgreSQL as datastore:
```ShellSession
$ docker create -e constraint:use==db --network=functions-network -v /var/lib/postgresql/data --name postgresql-data postgres /bin/true;
$ docker run -d -e constraint:use==db --network=functions-network --volumes-from postgresql-data --name functions-postgres -e POSTGRES_PASSWORD=mysecretpassword postgres
```

4. Start Oracle Functions:
```ShellSession
$ docker run -d --name functions-00 \
        -l functions \
        -e constraint:use==worker \
        --network=functions-network \
        -p 8080:8080 \
        -v /var/run/docker.sock:/var/run/docker.sock \
        -e 'MQ_URL=redis://functions-redis' \
        -e 'DB_URL=postgres://postgres:mysecretpassword@functions-postgres/?sslmode=disable' \
        treeder/functions
```

5. Load Balancer:

```ShellSession
$ export BACKENDS=$(docker ps --filter label=functions --format="{{ .ID }}" | xargs docker inspect | jq -r '.[].NetworkSettings.Ports["8080/tcp"][] | .HostIp + ":" + .HostPort' | paste -d, -s -)

$ docker run -d --name functions-lb -p 80:80 -e BACKENDS noqcks/haproxy

$ export FUNCTIONS=$(docker port functions-lb | cut -d ' ' -f3)

$ curl -H "Content-Type: application/json" -X POST -d '{ "app": { "name":"myapp" } }' http://$FUNCTIONS/v1/apps
{"message":"App successfully created","app":{"name":"myapp","config":null}}

$ curl -H "Content-Type: application/json" -X POST -d '{ "route": { "type": "sync", "path":"/hello-sync", "image":"funcy/hello" } }' http://$FUNCTIONS/v1/apps/myapp/routes
{"message":"Route successfully created","route":{"app_name":"myapp","path":"/hello-sync","image":"funcy/hello","memory":128,"type":"sync","config":null}}

$ curl -H "Content-Type: application/json" -X POST -d '{ "name":"Johnny" }' http://$FUNCTIONS/r/myapp/hello-sync
Hello Johnny!
```
