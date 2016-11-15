# IronFunctions Routes

Routes have a many-to-one mapping to an [app](apps.md).

A good practice to get the best performance on your IronFunctions API is define
the required memory for each function.

## Route level configuration

When creating a route, you can configure it to tweak its behavior, the possible
choices are: `memory`, `type` and `config`.

`memory` is number of usable MiB for this function. If during the execution it
exceeds this maximum threshold, it will halt and return an error in the logs. It
expects to be an integer. Default: `128`.

`type` is the type of the function. Either `sync`, in which the client waits
until the request is successfully completed, or `async`, in which the clients
dispatches a new request, gets a task ID back and closes the HTTP connection.
Default: `sync`.

`config` is a map of values passed to the route runtime in the form of
environment variables.

Note: Route level configuration overrides app level configuration.

TODO: link to swagger doc on swaggerhub after it's updated.

## Understanding IronFunctions memory management

When IronFunctions starts it registers the total available memory in your system
in order to know during its runtime if the system has the required amount of
free memory to run each function. Every function starts the runner reduces the
amount of memory used by that function from the available memory register. When
the function finishes the runner returns the used memory to the available memory
register.

### Creating function

```
curl -H "Content-Type: application/json" -X POST -d '{
    "route": {
        "path":"<route name>",
        "image":"<route image>",
        "memory": <memory mb number>,
        "type": "<route type>",
        "config": {"<unique key>": <value>}
    }
}' http://localhost:8080/v1/apps/<app name>/routes
```

Eg. Creating `/myapp/hello` with required memory as `100mb`, type `sync` and
some container configuration values.

```
curl -H "Content-Type: application/json" -X POST -d '{
    "route": {
        "path":"/hello",
        "image":"iron/hello",
        "memory": 100,
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
        "type": "<route type>",
        "config": {"<unique key>": <value>}
    }
}' http://localhost:8080/v1/apps/<app name>/routes/<route name>
```

Eg. Updating `/myapp/hello` required memory as `100mb`, type `async` and changed
container configuration values.

```
curl -H "Content-Type: application/json" -X POST -d '{
    "route": {
        "memory": 100,
        "type": "async",
        "config": {"APPLOG": "stdout"}
    }
}' http://localhost:8080/v1/apps/myapp/routes/hello
```