# Contributing to IronFunctions

We welcome all contributions!

## How to contribute

* Fork the repo
* Fix an issue or create an issue and fix it
* Create a Pull Request that fixes the issue
* Sign the CLA
* Good Job! Thanks for being awesome!

## Documentation

When creating a Pull Request, make sure that you also update the documentation
accordingly.

Most of the times, when making some behavior more explicit or adding a feature,
a documentation update is necessary.

You will either update a file inside docs/ or create one. Prefer the former over
the latter. If you are unsure, do not hesitate in open the PR with a comment
asking for suggestions on how to address the documentation part.

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
make docker-run
```

will start IronFunctions inside a Docker container.

## Tests in Docker

```sh
make docker-test

```

will test IronFunctions inside a Docker container.
