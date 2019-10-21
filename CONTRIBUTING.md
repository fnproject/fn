# Contributing to Fn

## Start Here

We're excited that you're interested in contributing to the Fn Project! Please see the general project [contributing guidelines](https://github.com/fnproject/docs/blob/master/community/CONTRIBUTING.md) page first!

## Fn Server (this repo) Specifics

The Fn Server is only for the essential parts of the entire Fn ecosystem.

These include:

- The core API (apps, routes, calls, logs)
- Executing functions (sync)
- Extension points (callbacks, middleware, API additions)

This does __not__ include:

- authentication
- stats/metrics
- special/optional features such as triggers, fdk's, workflows, event sources, etc.
- could be argued that additional I/O formats beyond the basic ones we support should be built as plugins too

Rule of thumb: If it could be built as an extension, then build it as an extension. 

We WILL accept any reasonable additions to extension points in order to support building extensions. 

We WILL do whatever we can to make it easy for users to add extensions (easy builds or use Go plugins). 

Graduation: Some extensions can graduate into core if they become commonplace in the community (ie: majority of users are using it). 

## How to Build and Run the Fn Server

### Build Dependencies ###
- [Go](https://golang.org/doc/install)
- [Dep](https://github.com/golang/dep)

### Getting the Repository ###

`$ git clone https://github.com/fnproject/fn.git`

See contributing guide for pointers about making changes to fn (ie forking).
Note that you must clone fnproject's repo, and not your fork, in order for the
build to work.

### Build

Requires Go >= 1.11.0 for go mod support

Change to the correct directory (if not already there):

`$ cd path/to/fnproject/fn`

Then after every change, run:

```sh
make run
```

or just build:

```sh
make build
```

This builds and runs the `fn` binary. It will start Fn using an embedded `sqlite3` database running on port `8080`.

### Test

```sh
make test
```

#### Run in Docker

Start Fn inside a Docker container:

```sh
make docker-run
```

Build the fn docker image, and then specify the version to the fn start cmd to start a local Fn server with the built image:
```sh
make docker-build
fn start --version latest
```

## Tests in Docker

Run tests inside a Docker container:

```sh
make docker-test

```
