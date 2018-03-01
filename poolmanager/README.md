# Running fn in split LB/API/runner mode

## Create certificates

This is a useful article to read for quickly generating mutual TLS certs:

http://www.levigross.com/2015/11/21/mutual-tls-authentication-in-go/

tl;dr: Get this https://github.com/levigross/go-mutual-tls/blob/master/generate_client_cert.go

add IP `127.0.0.1` to the cert by adding the line

```golang
	template.IPAddresses = append(template.IPAddresses, net.ParseIP("127.0.0.1"))
```

somewhere around line 124,
add the "net" package to the list of import packages and run it with

```bash
go run generate_client_cert.go --email-address a@a.com
```

Tada! Certs.

## Starting the components (as regular processes)

### API server

```bash
FN_NODE_TYPE=api ./fnserver
```

### Runner

```bash
mkdir /tmp/runnerdata
FN_NODE_TYPE=pure-runner FN_PORT=8082 FN_NODE_CERT=cert.pem FN_NODE_CERT_AUTHORITY=cert.pem FN_NODE_CERT_KEY=key.pem ./fnserver
```

### NuLB

```bash
mkdir /tmp/lbdata
FN_NODE_TYPE=lb FN_PORT=8081 FN_RUNNER_API_URL=http://localhost:8080 FN_NODE_CERT=cert.pem FN_NODE_CERT_AUTHORITY=cert.pem FN_NPM_ADDRESS=localhost:8083 FN_NODE_CERT_KEY=key.pem FN_LOG_LEVEL=DEBUG ./fnserver
```

### Node Pool Manager (NPM)

Currently the NPM uses a fixed, single-node instance of the Runner to simulate its "pool". The runner answers on port 8082 in this example,
but the GRPC port is 9190.

```bash
go build -o fnnpm poolmanager/server/main.go

FN_LOG_LEVEL=DEBUG \
FN_NODE_CERT=cert.pem  \
FN_NODE_CERT_KEY=key.pem  \
FN_NODE_CERT_AUTHORITY=cert.pem  \
FN_PORT=8083  \
FN_RUNNER_ADDRESS=127.0.0.1:9190 \
./fnnpm
```

### Directing a request to a specific LB Group

While the data model is extended to support LB Group metadata, requests can specify the LB Group mapping via the temporary extension header `X_FN_LB_GROUP_ID`. If not present, the LB Group ID will be set to _default_.

```bash
curl -H "X_FN_LB_GROUP_ID: noway" http://localhost:8081/r/myapp/hello
```

## Starting the components (in Docker containers)

### Build the images

The images don't yet exist in a registry, so they need building first.

```bash
docker build -f images/fnnpm/Dockerfile -t fnproject/fnnpm:latest .
docker build -f images/nulb/Dockerfile -t fnproject/nulb:latest .
docker build -f images/api/Dockerfile -t fnproject/api:latest .
docker build -f images/runner/Dockerfile -t fnproject/runner:latest .
```

### Start the containers

### API

This *shouldn't* need to talk to the Docker daemon, but it still tries to *for now*. So mount the socket.
```
docker run -d \
           -name api \
           -v /var/run/docker.sock:/var/run/docker.sock \
           -p 8080:8080 \
           fnproject/api:latest
```
### Node Pool Manager (NPM)
```
docker run -d \
           --name fnnpm \
           -e FN_RUNNER_ADDRESSES=docker.for.mac.localhost:9190,docker.for.mac.localhost:9191 \
           -p 8083:8080 \
           -v $(pwd)/cert.pem:/certs/cert.pem \
           -v $(pwd)/key.pem:/certs/key.pem \
           -e FN_NODE_CERT=/certs/cert.pem \
           -e FN_NODE_CERT_KEY=/certs/key.pem \
           -e FN_NODE_CERT_AUTHORITY=/certs/cert.pem \
           fnproject/fnnpm:latest
```

### NuLB

Again, this *shouldn't* need to talk to the Docker daemon, but it still tries to *for now*. So mount the socket.
```bash
docker run -d \
           --name nulb \
           -v /var/run/docker.sock:/var/run/docker.sock \
           -p 8081:8080 \
           -v $(pwd)/cert.pem:/certs/cert.pem \
           -v $(pwd)/key.pem:/certs/key.pem \
           -e FN_RUNNER_API_URL=http://docker.for.mac.localhost:8080 \
           -e FN_NPM_ADDRESS=docker.for.mac.localhost:8083 \
           -e FN_NODE_CERT=/certs/cert.pem \
           -e FN_NODE_CERT_KEY=/certs/key.pem \
           -e FN_NODE_CERT_AUTHORITY=/certs/cert.pem \
           fnproject/nulb:latest
```

#### First runner
```bash
docker run -d \
           --name runner \
           -v /var/run/docker.sock:/var/run/docker.sock \
           -p 9190:9190 \
           -e FN_GRPC_PORT=9190 \
           -p 8095:8080 \
           -v $(pwd)/cert.pem:/certs/cert.pem \
           -v $(pwd)/key.pem:/certs/key.pem \
           -e FN_NODE_CERT=/certs/cert.pem \
           -e FN_NODE_CERT_KEY=/certs/key.pem \
           -e FN_NODE_CERT_AUTHORITY=/certs/cert.pem \
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
           -v $(pwd)/cert.pem:/certs/cert.pem \
           -v $(pwd)/key.pem:/certs/key.pem \
           -e FN_NODE_CERT=/certs/cert.pem \
           -e FN_NODE_CERT_KEY=/certs/key.pem \
           -e FN_NODE_CERT_AUTHORITY=/certs/cert.pem \
           fnproject/runner:latest
```


