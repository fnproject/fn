FN integration API tests
======================================


Test dependencies
-----------------

```bash
DOCKER_HOST - for building images
API_URL - Fn API endpoint
```

How to run tests?
-----------------

```bash
export API_URL=http://localhost:8080
go test -v ./...
```
