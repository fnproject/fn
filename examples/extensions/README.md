# Extensions Example

This example adds extra endpoints to the API. See [main.go](main.go) for example code. 

## Building and Running

```sh
go build
./extensions
```

Then test with:

```sh
# First, create an app
fn apps create myapp
# And test
curl http://localhost:8080/v1/custom1
curl http://localhost:8080/v1/custom2
curl http://localhost:8080/v1/apps/myapp/custom3
curl http://localhost:8080/v1/apps/myapp/custom4
```
