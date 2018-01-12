# Fn Routes

Routes have a many-to-one mapping to an [app](apps.md).

A good practice to get the best performance on your Fn API is define
the required memory as well as CPU limits for each function.

## Route level configuration

When creating a route, you can configure it to tweak its behavior, the possible
choices are: `memory`, `cpus`, `type` and `config`.

`memory` is number of usable MiB for this function. If during the execution it
exceeds this maximum threshold, it will halt and return an error in the logs. It
expects to be an integer. Default: `128`.

`cpus` is the amount of available CPU cores for this function. For example, `100m` or `0.1`
will allow the function to consume at most 1/10 of a CPU core on the running machine. It
expects to be a string in MilliCPUs format ('100m') or floating-point number ('0.5').
Default: `""` or unset, which is unlimited.

`type` is the type of the function. Either `sync`, in which the client waits
until the request is successfully completed, or `async`, in which the clients
dispatches a new request, gets a call ID back and closes the HTTP connection.
Default: `sync`.

`config` is a map of values passed to the route runtime in the form of
environment variables.

Note: Route level configuration overrides app level configuration.

TODO: link to swagger doc on swaggerhub after it's updated.

## Understanding Fn memory and CPU management

When Fn starts it registers the total available memory and CPU cores in your system
in order to know during its runtime if the system has the required amount of
free memory and CPU to run each function. Every function starts the runner reduces the
amount of memory and CPU used by that function from the available memory and CPU register.
When the function finishes the runner returns the used memory and CPU to the available
memory and CPU register.

### Creating function

```
curl -H "Content-Type: application/json" -X POST -d '{
    "route": {
        "path":"<route name>",
        "image":"<route image>",
        "memory": <memory mb number>,
        "cpus": "<cpus MilliCPUs or floating-point number>",
        "type": "<route type>",
        "config": {"<unique key>": <value>}
    }
}' http://localhost:8080/v1/apps/<app name>/routes
```

Eg. Creating `/myapp/hello` with required memory as `100mb`, cpus as `200m`, type `sync` and
some container configuration values.

```
curl -H "Content-Type: application/json" -X POST -d '{
    "route": {
        "path":"/hello",
        "image":"fnproject/hello",
        "memory": 100,
        "cpus": "200m",
        "type": "sync",
        "config": {"APPLOG": "stderr"}
    }
}' http://localhost:8080/v1/apps/myapp/routes
```

### Updating function

```
curl -H "Content-Type: application/json" -X POST -d '{
    "route": {
        "memory": <memory mb number>,
        "cpus": "<cpus MilliCPUs or floating-point number>",
        "type": "<route type>",
        "config": {"<unique key>": <value>}
    }
}' http://localhost:8080/v1/apps/<app name>/routes/<route name>
```

Eg. Updating `/myapp/hello` required memory as `100mb`, cpus as `0.2`, type `async` and changed
container configuration values.

```
curl -H "Content-Type: application/json" -X POST -d '{
    "route": {
        "memory": 100,
        "cpus": "0.2",
        "type": "async",
        "config": {"APPLOG": "stdout"}
    }
}' http://localhost:8080/v1/apps/myapp/routes/hello
```
