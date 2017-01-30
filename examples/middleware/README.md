# Middleware Example

This example adds a simple authentication middleware to IronFunctions. See [main.go](main.go) for example code. 

## Building and Running

```sh
go build
./middleware
```

Then test with:

```sh
# First, create an app
fn apps create myapp
# And test
curl http://localhost:8080/v1/apps
```

You should get a 401 error. 

Add an auth header and it should go through successfully:

```sh
curl -X GET -H "Authorization: Bearer KlaatuBaradaNikto" http://localhost:8080/v1/apps
```
