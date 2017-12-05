# Contributing to Fn

We welcome all contributions!

## Rules of Fn core (ie: what belongs here and what doesn't)

Fn server (core) is only for the essential parts of the entire Fn ecosystem. 
These include:

- The core API (apps, routes, calls, logs)
- Executing functions (sync and async)
- Extension points (callbacks, middleware, API additions)

That's it. Everything else should be built as an extension.

This does not include:

- authentication
- stats/metrics
- special/optional features such as triggers, fdk's, workflows, event sources, etc.
- could be argued that additional I/O formats beyond the basic ones we support should be built as plugins too

Rule of thumb: If it could be built as an extension, then build it as an extension. 

We WILL accept any reasonable additions to extension points in order to support building extensions. 

We WILL do whatever we can to make it easy for users to add extensions (easy builds or use Go plugins). 

Graduation: Some extensions can graduate into core if they become commonplace in the community (ie: majority of users are using it). 

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

Requires Go >=1.9.1.

The first time after you clone or after dependencies get updated, run:

```sh
make dep
```

Then after every change, run

```sh
make run
```

to build and run the `functions` binary.  It will start Functions using an embedded `sqlite3` database running on port `8080`.

### Test

```sh
make test
```

#### Run in Docker

```sh
make docker-run
```

will start Functions inside a Docker container.

## Tests in Docker

```sh
make docker-test

```

will test Functions inside a Docker container.
