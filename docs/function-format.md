# Open Function Format

This document will describe the details of how a function works, inputs/outputs, etc.

## I/O Formats

### STDIN and Environment Variables

While wanting to keep things simple, flexible and expandable, we decided to go back to the basics, using Unix input and output. Standard in is easy to use in any language and doesn't require anything extra. It also allows streaming input so we can do things like keeping a container running some time and stream requests into the container.

Configuration values, environment information and other things will be passed in through environment variables.

The goals of the I/O format are the following:

* Very easy to use and parse
* Supports hot for increasing performance (more than one call per container execution)
* Ability to build higher level abstractions on top (ie: Lambda syntax compatible)

The format is still up for discussion and in order to move forward and remain flexible, it's likely we will just allow different I/O formats and the function creator can decide what they want, on a per function basis. Default being the simplest format to use.

TODO: Put common env vars here, that show up in all formats.

#### Default I/O Format

The default format is simply the request body itself plus some environment variables. For instance, if someone were to post a JSON body, the unmodified body would be sent in via STDIN. The result comes via STDOUT. When task is done, pipes are closed and the container running the function is terminated.

Pros:

* Very simple to use

Cons:

* Not very efficient resource utilization - one function for one event.

#### HTTP I/O Format

`format: http`

HTTP format could be a good option as it is in very common use obviously, most languages have some semi-easy way to parse it, and it supports hot format. The response will look like a HTTP response. The communication is still done via stdin/stdout, but these pipes are never closed unless the container is explicitly terminated. The basic format is:

Request:

```text
GET / HTTP/1.1
Content-Length: 5

world
```

Response:

```text
HTTP/1.1 200 OK
Content-Length: 11

hello world
```

The header keys and values would be populated with information about the function call such as the request URL and query parameters.

`Content-Length` is determined by the [Content-Length](https://tools.ietf.org/html/rfc7230#section-3.3.3) header, which is mandatory both for input and output. It is used by Functions to know when stop writing to STDIN and reading from STDOUT.

Pros:

* Supports streaming
* Common format

Cons:

* Requires a parsing library or fair amount of code to parse headers properly
* Double parsing - headers + body (if body is to be parsed, such as json)

#### JSON I/O Format

`format: json`

The JSON format is a nice hot format as it is easy to parse in most languages.

If a request comes in like this:

```json
{
  "some": "input"
}
```

Internally function receives data in following format:

```json
{
  "body": "{\"some\":\"input\"}",
  "request_url": "http://localhost:8080/r/myapp/myfunc?q=hi",
  "headers": {
    "Content-Type": "application/json"
  },
  "config": {
    "MY_KEY": "something",
  }
}
```

Each request will be separated by a new line.

Function's output format should have the following format:

```json
{
  "status_code": 200,
  "body": "{\"some\":\"output\"}",
  "headers": {
    "Content-Type": "application/json"
  }
}
```

Only body is required.

At client side user will receive HTTP response with HTTP headers, status code and the body from taken from function's response.

Pros:

* Supports hot format
* Easy to parse

Cons:

* Not streamable

## Output

### Output back to client

Typically JSON is the output format and is the default output, but any format can be used.

### Logging

Standard error is reserved for logging, like it was meant to be. Anything you output to STDERR will show up in the logs. And if you use a log
collector like logspout, you can collect those logs in a central location. See [logging](logging.md).
