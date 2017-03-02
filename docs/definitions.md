# API Definitions

# Applications

Applications are the top level object that groups routes together to create an API.

### Creating applications

Using `fn`:

```sh
fn apps create --config k1=v1 --config k2=v2 myapp
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

### App Example

```json
{
    "name": "myapp",
    "myconfig": "config"
}
```

### Properties

#### name (string)

`name` is a property that references an unique app.

App names are immutable. When updating apps with `PATCH` requests, keep in mind that although you
are able to update an app's configuration set, you cannot really rename it.

#### config (object)

`config` is a map of values passed to the route runtime in the form of
environment variables.

Note: Route level configuration overrides app level configuration.

## Routes

### Creating routes

Using `fn`:

```sh
fn routes create myapp /path --config k1=v1 --config k2=v2 --image iron/hello
```

Or using a cURL:

```sh
curl -H "Content-Type: application/json" -X POST -d '{
    "app": {
        "path": "/path",
        "image": "image",
        "config": {
            "k1": "v1",
            "k2": "v2"
        }
    }
}' http://localhost:8080/v1/apps/myapp/routes
```

### Route Example
```json
{
    "path": "/hello",
    "image": "iron/hello",
    "type": "sync",
    "memory": 128,
    "config": {
        "key": "value",
        "key2": "value2",
        "keyN": "valueN",
    },
    "headers": {
        "content-type": [
            "text/plain"
        ]
    }
}
```

### Properties

#### path (string)

Represents a unique `route` in the API and is identified by the property `path` and `app`.

Every `route` belongs to an `app`.

Note: Route paths are immutable. If you need to change them, the appropriate approach
is to add a new route with the modified path.

#### image (string)

`image` is the name or registry URL that references to a valid container image located locally or in a remote registry (if provided any registry address).

If no registry is provided and image is not available locally the API will try pull it from a default public registry.

#### type (string)

Options: `sync` and `async`

`type` is defines how the function will be executed. If type is `sync` the request will be hold until the result is ready and flushed.

In `async` functions the request will be ended with a `call_id` and the function will be executed in the background.

#### memory (number)

`memory` defines the amount of memory (in megabytes) required to run this function.

#### config (object of string values)

`config` is a map of values passed to the route runtime in the form of
environment variables.

Note: Route level configuration overrides app level configuration.

#### headers (object of array of string)

`header` is a set of headers that will be sent in the function execution response. The header value is an array of strings.

#### format (string)

`format` defines if the function is running or not in `hot function` mode.

To define the function execution as `hot function` you set it as one of the following formats:

- `"http"`

### 'Hot function' Only Properties

This properties are only used if the function is in `hot function` mode

#### max_concurrency (string)

This property defines the maximum amount of concurrent hot functions instances the function should have (per IronFunction node).