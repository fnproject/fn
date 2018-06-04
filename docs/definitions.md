# API Definitions

# Applications

Applications are the top level object that groups routes together to create an API.

### Creating applications

Using `fn`:

```sh
fn create app --config k1=v1 --config k2=v2 myapp
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
fn create route myapp /path --config k1=v1 --config k2=v2 --image fnproject/hello
```

Or using cURL:

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
    "image": "fnproject/hello",
    "type": "sync",
    "memory": 128,
    "cpus": "100m",
    "config": {
        "key": "value",
        "key2": "value2",
        "keyN": "valueN"
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

#### cpus (string)

`cpus` defines the amount of CPU cores (in MilliCPUs or floating-point number) required to run this function. For example, `500m` for 1/2 CPU cores or `0.5` for 1/2 CPU cores.

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


## Calls and their statuses

### Sync/Async Call statuses

With each function call, no matter would that be sync or async server makes a record of this it.
While execution async function server returns `call_id`:
 
```json
 {
    "call_id": "f5621e8b-725a-4ba9-8323-b8cdc02ce37e"
 }
```
that can be used to track call status using following command:
 
```sh
 
 curl -v -X GET ${FN_API_URL}/v1/calls/f5621e8b-725a-4ba9-8323-b8cdc02ce37
 
```

```json
{
    "message": "Successfully loaded call",
    "call": {
        "id": "f5621e8b-725a-4ba9-8323-b8cdc02ce37e",
        "status": "success",
        "completed_at": "2017-06-02T15:31:30.887+03:00",
        "created_at": "2017-06-02T15:31:30.597+03:00",
        "started_at": "2017-06-02T15:31:30.597+03:00",
        "app_name": "newapp",
        "path": "/envsync"
    }
}

```

Server response contains timestamps(created, started, completed) and execution status for this call.

For sync call `call_id` can be retrieved from HTTP headers:
```sh
curl -v localhost:8080/r/newapp/envsync 
*   Trying ::1...
* TCP_NODELAY set
* Connected to localhost (::1) port 8080 (#0)
> GET /r/newapp/envsync HTTP/1.1
> Host: localhost:8080
> User-Agent: curl/7.51.0
> Accept: */*
> 
< HTTP/1.1 200 OK
< Fn_call_id: f5621e8b-725a-4ba9-8323-b8cdc02ce37e
< Date: Fri, 02 Jun 2017 12:31:30 GMT
< Content-Length: 489
< Content-Type: text/plain; charset=utf-8
< 
...
```
Corresponding HTTP header is `Fn_call_id`.

### Per-route calls

In order get list of per-route calls please use following command:

```sh
curl -X GET ${FN_API_URL}/v1/app/{app}/calls/{route}

```
Server will replay with following JSON response:

```json
{
    "message": "Successfully listed calls",
    "calls": [
        {
            "id": "80b12325-4c0c-5fc1-b7d3-dccf234b48fc",
            "status": "success",
            "completed_at": "2017-06-02T15:31:22.976+03:00",
            "created_at": "2017-06-02T15:31:22.691+03:00",
            "started_at": "2017-06-02T15:31:22.691+03:00",
            "app_name": "newapp",
            "path": "/envsync"
        },
        {
            "id": "beec888b-3868-59e3-878d-281f6b6f0cbc",
            "status": "success",
            "completed_at": "2017-06-02T15:31:30.887+03:00",
            "created_at": "2017-06-02T15:31:30.597+03:00",
            "started_at": "2017-06-02T15:31:30.597+03:00",
            "app_name": "newapp",
            "path": "/envsync"
        }
    ]
}
```

### Pagination

The fn api utilizes 'cursoring' to paginate large result sets on endpoints
that list resources. The parameters are read from query parameters on incoming
requests, and a cursor will be returned to the user if they receive a full
page of data to use to retrieve the next page. We'll walk through with a
concrete example in just a minute.

To begin paging through a results set, a user should provide a `?cursor` with an
empty string or omit the cursor query parameter altogether. A user may specify
how many results per page they would like to receive with the `?per_page`
query parameter, which defaults to 30 and has a max of 100. After calling a
list endpoint, a user may receive a `response.next_cursor` value in the
response, next to the list of resources. If `next_cursor` is an empty string,
then there is no further data to retrieve and the user may stop paging. If
`next_cursor` is a non-empty string, the user may provide it in the next
request's `?cursor` parameter to receive the next page.

briefly, what this means, is user code should look similar to this:

```
req = "http://my.fn.com/v1/apps/"
cursor = ""

for {
  req_with_cursor = req + "?" + cursor
  resp = call_http(req_with_cursor)
  do_things_with_apps(resp["apps"])

  if resp["next_cursor"] == "" {
    break
  }
  cursor = resp["next_cursor"]
}

# done!
```

client libraries will have variables for each of these variables in their
respective languages to make this a bit easier, but may the for be with
you.

