# Fn container contract

NOTE: THIS IS WORK IN PROGRESS AND ITS API IS SUBJECT TO CHANGE

This document will describe the details of how a function works, inputs/outputs, etc.
(It is meant to replace (./function-format.md) - if the time has come, remove this line,
for now it defines the 'http-stream' format option)

The basic idea is to handle http requests over a unix domain socket. Each
container will only receive one request at a time, in that until a response is
returned from a previous request, no new requests will be issued to a
container's http server.

### XXX(reed): motivation section for future us posterity?

### FDK Contract outline

Function Development Kits (FDKs) are libraries for various languages that implement Fn's container contract for function input, output and configuration. In order to be a fully 'FDK compliant', below are the rules:

If `FN_FORMAT` variable is http-stream, then FDKs __MUST__ parse `FN_LISTENER` environment variable.

`FN_LISTENER` must contain the listener address for FDKs to bind/listen to. Currently we only support unix-domain-socket (UDS) with `SOCK_STREAM` (TCP). This means `FN_LISTENER` prefix is always `unix:/`.

For example:

```
FN_LISTENER=unix:/var/run/fn/listener/listen.sock
FN_FORMAT=http-stream
```

If `FN_FORMAT` is `http-stream`, then absence of `FN_LISTENER` or "unix:" prefix in `FN_LISTENER` is an error and FDKs are REQUIRED to terminate/exit with error.

Before exiting, FDKs __SHOULD__ remove the UDS file (from `FN_LISTENER` path).

FDKs upon creation of UDS file on disk with bind system call __SHOULD__ be ready to receive and handle traffic. Upon bind call, the UDS file __MUST__ be writable by fn-agent. In order to create a properly permissioned UDS file, FDKs __MUST__ create a file with [at least] permissions of `0666`, if the language provides support for creating this file with the right permissions this may be easily achieved; users may alternatively bind to a file that is not `FN_LISTENER`, modify its permissions to the required setting, and then symlink that file to `FN_LISTENER` (see fdk-go for an example).

Path in `FN_LISTENER` (after "unix:" prefix) cannot be larger than 107 bytes.

FDKs __MUST__ listen on the unix socket within 5 seconds of startup, Fn will enforce time limits and will terminate such FDK containers.

Once initialised the FDK should respond to HTTP requests by accepting connections on the created unix socket.

The FDK __SHOULD NOT__ enforce any read or write timeouts on incoming requests or responses

The FDK __SHOULD__ support HTTP/1.1 Keep alive behaviour

The agent __MUST__ maintain no more than one concurrent HTTP connection to the FDK HTTP servers

Containers __MUST__ implement HTTP/1.1

Any data sent to Stdout and Stderr will be logged by Fn and sent to any configured logging facility

Each function container is responsible for handling multiple function
invocations against it, one at a time, for as long as the container lives.

Fn will make HTTP requests to the container on the `/call` URL of the containers HTTP UDS port using.

```
POST /call HTTP/1.1
Host: localhost:8080
Fn-Call-Id : <call_id>
Fn-Deadline: <date/time>
Content-type: application/cloudevent+json

<Body here>
```

```
HTTP/1.1 200 OK
Fn-Http-Status: 204
Fn-Http-H-My-Header: foo
Content-type: text/plain

<Body here>
```

### Environment Variables

The below are the environment variables that a function can expect to use.
FDKs __SHOULD__ provide a facility to easily access these without having to
use an OS library.

* `FN_ID` - fn id
* `FN_APP_ID` - app id of the fn
* `FN_NAME` - name of the fn
* `FN_APP_NAME` - the name of the application of the fn
* `FN_FORMAT` - `http-stream` (DEPRECATED)
* `FN_LISTENER` - the path where a unix socket file should accept
* `FN_MEMORY` - a number representing the amount of memory available to the call, in MB
* `FN_TMPSIZE` - a number representing the amount of writable disk space available under /tmp for the call, in MB

In addition to these, all config variables set on `app.config` or `fn.config` will be populated into the environment exactly as configured, for example if `app.config = { "HAMMER": "TIME" }` your environment will be populated with `HAMMER=TIME`.

### Headers

* `Fn-Call-Id` - id for the call
* `Fn-Deadline` - RFC3339 timestamp indicating the deadline for a function call
* `Fn-*` - reserved for future usage
* `*` - any other headers, potentially rooted from an http trigger

