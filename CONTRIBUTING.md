# Contributing to IronFunctions

We welcome all contributions!

## How to contribute

* Fork the repo
* Fix an issue or create an issue and fix it
* Create a Pull Request that fixes the issue
* Sign the CLA
* Good Job! Thanks for being awesome!

## How to build and get up and running

### Build

The first time after you fork or after dependencies get updated, run:

```sh
make dep
```

Then after every change, run:

```sh
make build
```

to build the `functions` binary.

### Run

```sh
./functions
```

will start IronFunctions using an embedded `Bolt` database running on port `8080`.

### Test

```sh
make test
```

#### Run in Docker

```sh
make run-docker
```

will start IronFunctions inside a Docker container.

## Tests in Docker

```sh
make test-docker

```

will test IronFunctions inside a Docker container.
