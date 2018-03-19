FN integration API tests
======================================


These are tests that can either run locally against the current codebase (e.g. in an IDE)  or remotely against a  running Fn instance.


Test dependencies
-----------------

```bash
DOCKER_HOST - for building images
FN_API_URL - Fn API endpoint - leave this unset to test using the local codebase
```

How to run tests?
-----------------

```bash
export FN_API_URL=http://localhost:8080
go test -v ./...
```
