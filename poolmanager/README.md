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

## Starting the components

### API server

```bash
FN_NODE_TYPE=api ./fnserver
```

### Runner

```bash
mkdir /tmp/runnerdata
FN_NODE_TYPE=pure-runner FN_PORT=8082 FN_NODE_CERT=cert.pem FN_NODE_CERT_AUTHORITY=cert.pem FN_NODE_CERT_KEY=key.pem ./fnserver
```

### LB

```bash
mkdir /tmp/lbdata
FN_NODE_TYPE=lb FN_PORT=8081 FN_RUNNER_API_URL=http://localhost:8080 FN_NODE_CERT=cert.pem FN_NODE_CERT_AUTHORITY=cert.pem FN_NPM_ADDRESS=localhost:8083 FN_NODE_CERT_KEY=key.pem FN_LOG_LEVEL=DEBUG ./fnserver
```

### NPM

Currently the NPM uses a fixed, single-node instance of the Runner to simulate its "pool". The runner answers on port 8082 in this example,
but the GRPC port is 9120.

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
