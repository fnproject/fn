# Running load-balanced fn against a pool of runners

## Motivation

You can run a load-balanced setup for fn to route requests to a group of one or more runners.

## Starting the components (as regular processes)

### API server

```bash
FN_NODE_TYPE=api ./fnserver
```

### Runners

```bash
mkdir /tmp/runnerdata
# first runner
FN_NODE_TYPE=pure-runner FN_PORT=8082 FN_GRPC_PORT=9190 ./fnserver
# on another terminal, start a second runner
FN_NODE_TYPE=pure-runner FN_PORT=8083 FN_GRPC_PORT=9191 ./fnserver
```

### LB

```bash
mkdir /tmp/lbdata
FN_NODE_TYPE=lb FN_PORT=8081 FN_RUNNER_API_URL=http://localhost:8080 FN_RUNNER_ADDRESSES=localhost:9190,localhost:9191 FN_LOG_LEVEL=DEBUG ./fnserver
```

## Starting the components (in Docker containers)

### Build the images

The images don't yet exist in a registry, so they need building first.

```bash
docker build -f images/lb/Dockerfile -t fnproject/lb:latest .
docker build -f images/api/Dockerfile -t fnproject/api:latest .
docker build -f images/runner/Dockerfile -t fnproject/runner:latest .
```

### Start the containers

This mode assumes that LB is started with a static set of runners in a single global pool. Note that this configuration does not support runner certificates and is that the communication between LB and runners is unencrypted.

### API

```
docker run -d \
           --name api \
           -v /var/run/docker.sock:/var/run/docker.sock \
           -p 8080:8080 \
           fnproject/api:latest
```

#### First runner
```bash
docker run -d \
           --name runner \
           -v /var/run/docker.sock:/var/run/docker.sock \
           -p 9190:9190 \
           -e FN_GRPC_PORT=9190 \
           -p 8095:8080 \
           fnproject/runner:latest
```

#### Second runner
```bash
docker run -d \
           --name runner-2 \
           -v /var/run/docker.sock:/var/run/docker.sock \
           -p 9191:9191 \
           -e FN_GRPC_PORT=9191 \
           -p 8096:8080 \
           fnproject/runner:latest
```

### LB

Retrieve the IP addresses for the runners and the API:

```bash
export RUNNER1=`docker inspect --format '{{ .NetworkSettings.IPAddress }}' runner`
export RUNNER2=`docker inspect --format '{{ .NetworkSettings.IPAddress }}' runner-2`
export API=`docker inspect --format '{{ .NetworkSettings.IPAddress }}' api`

```

Pass in the static set of runners to _FN\_RUNNER\_ADDRESSES_:

```bash
docker run -d \
           --name lb \
           -v /var/run/docker.sock:/var/run/docker.sock \
           -p 8081:8080 \
           -e FN_RUNNER_API_URL=http://$API:8080 \
           -e FN_RUNNER_ADDRESSES=$RUNNER1:9190,$RUNNER2:9191 \
           fnproject/lb:latest
```
