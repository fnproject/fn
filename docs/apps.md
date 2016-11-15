# Applications

Applications are the top level object that groups routes together to create an API.

## App level configuration

When creating or updating an app, you can pass in a map of config variables.

`config` is a map of values passed to the route runtime in the form of
environment variables.

Note: Route level configuration overrides app level configuration.

```sh
fnctl apps create --config k1=v1 --config k2=v2 myapp
```

Or using a cURL:

```sh
curl -H "Content-Type: application/json" -X POST -d '{
    "app": {
        "name":"myapp-curl",
        "config": {
            "k1": "v1",
            "k2": "v2"
        }
    }
}' http://localhost:8080/v1/apps
```