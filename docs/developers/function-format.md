# Open Function Format

This document will describe the details of how a function works, inputs/outputs, etc.

## I/O Formats

### STDIN and Environment Variables

While wanting to keep things simple, flexible and expandable, we decided to go back to the basics, using Unix input and output. Standard in is easy to use in any language and doesn't require anything extra. It also allows streaming input so we can do things like keeping a container running some time and stream requests into the container.

Configuration values, environment information and other things will be passed in through environment variables.

The goals of the I/O formats are the following:

* Very easy to use and parse
* Supports hot for increasing performance (more than one call per container execution)
* Ability to build higher level abstractions on top (ie: Lambda syntax compatible)

The format is still up for discussion and in order to move forward and remain flexible, it's likely we will just allow different I/O formats and the function creator can decide what they want, on a per function basis. Default being the simplest format to use.

The way that input data is supplied to functions depends on the input format (as specified in `func.yml`) that your function is using.

#### Environment Variables used by All Formats

Your function has access to a set of environment variables, independent of
the function's format:

* `FN_APP_NAME` - the name of the application that matched this route, eg: `myapp`
* `FN_PATH` - the matched route, eg: `/hello`
* `FN_METHOD` - the HTTP method for the request, eg: `GET` or `POST`
* `FN_FORMAT` - a string representing one of the [function formats](function-format.md), currently either `default` or `http`. Default is `default`.
* `FN_TYPE` - the type for this call, currently 'sync' or 'async'
* `FN_MEMORY` - a number representing the amount of memory available to the call, in MB
* `FN_CPUS` - a string representing the amount of CPU available to the call, in MilliCPUs or floating-point number, eg. `100m` or `0.1`. Header is present only if `cpus` is set for the route.
* `FN_TMPSIZE` - a number representing the amount of writable disk space available under /tmp for the call, in MB

In addition to these, all config variables set on `app.config` or `route.config` will be populated into the environment exactly as configured, for example if `app.config = { "HAMMER": "TIME" }` your environment will be populated with `HAMMER=TIME`.

### Default I/O Format

The default format is simply the request body itself plus some environment variables. For instance, if someone were to post a JSON body, the unmodified body would be sent in via STDIN. The result comes via STDOUT. When task is done, pipes are closed and the container running the function is terminated.

#### Default Format Env Vars

For `default` format, the following environment variables will be available:

* `FN_DEADLINE` - RFC3339 time stamp of the expiration (deadline) date of function execution.
* `FN_REQUEST_URL` - the full URL for the request ([parsing example](https://github.com/fnproject/fn/tree/master/examples/tutorial/params))
* `FN_CALL_ID` - a unique ID for each function execution.
* `FN_METHOD` - http method used to invoke this function
* `FN_HEADER_$X` - the HTTP headers that were set for this request. Replace $X with the upper cased name of the header and replace dashes in the header with underscores.
  * `$X` - $X is the header that came in the http request that invoked this function.

#### Pros/Cons

Pros:

* Very simple to use

Cons:

* Not very efficient resource utilization - one new container execution per event.

### CloudEvent I/O Format

`format: cloudevent`

The CloudEvent format is a nice hot format as it is easy to parse in most languages. It's also a [CNCF
defined format for events](https://github.com/cloudevents/spec/blob/master/spec.md).

If a request comes in with the following body:

```json
{
  "some": "input"
}
```

then, the input will be:

#### Input

Internally functions receive data in the example format below:

```json
{
  "cloudEventsVersion": "cloudEventsVersion value",
  "eventID": "the call ID",
  "source": "source value",
  "eventType": "eventType value",
  "eventTypeVersion": "eventTypeVersion value",
  "eventTime": "eventTime value",
  "schemaURL": "schemaURL value",
  "contentType": "contentType",
  "extensions": {
    "deadline":"2018-01-30T16:52:39.786Z",
    "protocol": {
      "type": "http",
      "method": "POST",
      "request_url": "http://localhost:8080/r/myapp/myfunc?q=hi",
      "headers": {
        "Content-Type": ["application/json"],
        "Other-Header": ["something"]
      }
    }
  },
  "data": {
    "some": "input"
  }
}
{
  NEXT EVENT
}
```

* data - the main contents. If HTTP is the protocol, this would be the request body.
* eventID - the unique ID for the event/call.
* contentType - format of the `data` parameter.
* extensions:
  * deadline - a time limit for the call, based on function timeout.
  * protocol - arbitrary map of protocol specific data. The above example shows what the HTTP protocol handler passes in. Subject to change and reduces reusability of your functions. **USE AT YOUR OWN RISK**. Under `protocol` => `headers` contains all of the HTTP headers exactly as defined in the incoming request.


#### Output

Function's output should be a copy of the input CloudEvent with any fields changed:

```json
{
  "cloudEventsVersion": "cloudEventsVersion value",
  "eventID": "the call ID",
  "source": "source value",
  "eventType": "eventType value",
  "eventTypeVersion": "eventTypeVersion value",
  "eventTime": "eventTime value",
  "schemaURL": "schemaURL value",
  "contentType": "contentType",
  "extensions": {
    "deadline":"2018-01-30T16:52:39.786Z",
    "protocol": {
      "type": "http",
      "status_code": 200,
      "headers": {
        "Other-Header": ["something"],
        
        
      }
    }
  },
  "data": {
    "some": "output"
  }
}
{
  NEXT OUTPUT OBJECT
}
```

* data - required - the response body.
* contentType - optional - format of `data`. Default is application/json.
* protocol - optional - protocol specific response options. Entirely optional. Contents defined by each protocol.

### JSON I/O Format

`format: json`

The JSON format is a nice hot format as it is easy to parse in most languages.

If a request comes in with the following body:

```json
{
  "some": "input"
}
```

then, the input will be:

#### Input

Internally functions receive data in the example format below:

```json
{
  "call_id": "123",
  "content_type": "application/json",
  "deadline":"2018-01-30T16:52:39.786Z",
  "body": "{\"some\":\"input\"}",
  "protocol": {
    "type": "http",
    "method": "POST",
    "request_url": "http://localhost:8080/r/myapp/myfunc?q=hi",
    "headers": {
      "Content-Type": ["application/json"],
      "Other-Header": ["something"]
    }
  }
}
{
  NEXT INPUT OBJECT
}
```

* body - the main contents. If HTTP is the protocol, this would be the request body.
* call_id - the unique ID for the call.
* content_type - format of the `body` parameter.
* deadline - a time limit for the call, based on function timeout.
* protocol - arbitrary map of protocol specific data. The above example shows what the HTTP protocol handler passes in. Subject to change and reduces reusability of your functions. **USE AT YOUR OWN RISK**.

Under `protocol`, `headers` contains all of the HTTP headers exactly as defined in the incoming request.

#### Output

Function's output format should have the following format:

```json
{
  "body": "{\"some\":\"output\"}",
  "content_type": "application/json",
  "protocol": {
    "status_code": 200,
    "headers": {
      "Other-Header": ["something"]
    }
  }
}
{
  NEXT OUTPUT OBJECT
}
```

* body - required - the response body.
* content_type - optional - format of `body`. Default is application/json.
* protocol - optional - protocol specific response options. Entirely optional. Contents defined by each protocol.

#### Pros/Cons of JSON format

Pros:

* Supports hot format
* Easy to parse

Cons:

* Not streamable

### HTTP I/O Format

`format: http`

HTTP format could be a good option as it is in very common use obviously, most languages have some semi-easy way to parse it, and it supports hot format. The response will look like a HTTP response. The communication is still done via stdin/stdout, but these pipes are never closed unless the container is explicitly terminated.

#### Request

```text
GET / HTTP/1.1
Content-Length: 5

world
```

#### Input

The input to the function will be in standard HTTP format, similar to the incoming request, but with the
following additional headers:

* `Fn_deadline` - RFC3339 time stamp of the expiration (deadline) date of function execution.
* `Fn_request_url` - the full URL for the request ([parsing example](https://github.com/fnproject/fn/tree/master/examples/tutorial/params))
* `Fn_call_id` - a unique ID for each function execution.
* `Fn_method` - the HTTP method used to invoke
* `$X` - the HTTP headers that were set for this request, exactly as they were sent in the request.

#### Output

Your function should output the exact response in HTTP format you'd like to be returned to the client:

```text
HTTP/1.1 200 OK
Content-Length: 11

hello world
```

#### Pros/Cons of HTTP Format

Pros:

* Supports streaming
* Common format

Cons:

* Requires a parsing library or fair amount of code to parse headers properly
* Double parsing - headers + body (if body is to be parsed, such as json)

## Logging

Standard out is where you should write response data for synchronous functions. Standard error
is where you should write for logging, as [it was intended](http://www.jstorimer.com/blogs/workingwithcode/7766119-when-to-use-stderr-instead-of-stdout).
And if you use a log collector like logspout, you can collect those logs in a central location. See [logging](../operating/logging.md).

So to write output to logs, simply log to STDERR. Here are some examples in a few languages.

In Go, simply use the [log](https://golang.org/pkg/log/) package, it writes to STDERR by default.

```go
log.Println("hi")
```

In Node.js:

```node
console.error("hi");
```

[More details for Node.js here](http://stackoverflow.com/a/27576486/105562).

In Ruby:

```ruby
STDERR.puts("hi")
```
