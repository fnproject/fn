# Extensions Example

This example adds extra endpoints to the API. See [main.go](main.go) for example code.

## Building and Running

```sh
go build
./extensions
```

First create an app `myapp`. Then test with:

```sh
curl http://localhost:8080/v1/custom1
curl http://localhost:8080/v1/custom2
curl http://localhost:8080/v1/apps/myapp/custom3
curl http://localhost:8080/v1/apps/myapp/custom4
```
