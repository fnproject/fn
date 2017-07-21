Oracle Functions integration API tests
======================================


Test dependencies
-----------------

```bash
DOCKER_HOST - for building images
API_URL - Oracle Functions API endpoint
```

How to run tests?
-----------------

```bash
export API_URL=http://localhost:8080
go test -v ./...
```
