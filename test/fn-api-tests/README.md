FN integration API tests
======================================


These are tests that can either run locally (e.g. in an IDE) using the local codebase to instantiate a server or remotely


Test dependencies
-----------------

```bash
DOCKER_HOST - for building images
FN_API_URL - Fn API endpoint -
```

How to run tests?
-----------------

```bash
export FN_API_URL=http://localhost:8080
go test -v ./...
```
