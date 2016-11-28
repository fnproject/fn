# Open Function Format

This document will describe the details of how a function works, inputs/outputs, etc.

## Formats

### STDIN and Environment Variables

While wanting to keep things simple, flexible and expandable, we decided to go back to the basics, using Unix input and output. Standard in is easy to use in any language and doesn't require anything extra. It also allows streaming input so we can do things like keeping a container running some time and stream requests into the container.

Configuration values, environment information and other things will be passed in through environment variables.

The goals of the input format are the following:

* Very easy to use and parse
* Streamable for increasing performance (more than one call per container execution)
* Ability to build higher level abstractions on top (ie: Lambda syntax compatible)

The format is still up for discussion and in order to move forward and remain flexible, it's likely we will just allow different input formats and the function creator can decide what they want, on a per function basis. Default being the simplest format to use.

#### Default I/O Format

The default I/O format is simply the request body itself plus some environment variables. For instance, if someone were to post a JSON body, the unmodified body would be sent in via STDIN. The result comes via STDOUT. When task is done, pipes are closed and the container running the function is terminated.

Pros:

* Very simple to use

Cons:

* Not streamable

#### HTTP I/O Format

`--format http`

HTTP format could be a good option as it is in very common use obviously, most languages have some semi-easy way to parse it, and it's streamable. The response will look like a HTTP response. The communication is still done via stdin/stdout, but these pipes are never closed unless the container is explicitly terminated. The basic format is:

Request:
```
GET / HTTP/1.1
Content-Length: 5

world
```

Response:
```
HTTP/1.1 200 OK
Content-Length: 11

hello world
```

The header keys and values would be populated with information about the function call such as the request URL and query parameters.

`Content-Length` is determined by the [Content-Length](https://tools.ietf.org/html/rfc7230#section-3.3.3) header, which is mandatory both for input and output. It is used by IronFunctions to know when stop writing to STDIN and reading from STDOUT.

Pros:

* Streamable
* Common format

Cons:

* Requires a parsing library or fair amount of code to parse headers properly
* Double parsing - headers + body (if body is to be parsed, such as json)

#### JSON I/O Format (not implemented)

`--format json`

The idea here is to keep the HTTP base structure, but make it a bit easier to parse by making the `request line` and `headers` a JSON struct.
Eg:

```
{
  "request_url":"http://....",
  "params": {
    "blog_name": "yeezy"
  }
}
BLANK LINE
BODY
```

Pros:

* Streamable
* Easy to parse headers

Cons:

* New, unknown format

### STDERR

Standard error is reserved for logging, like it was meant to be. Anything you output to STDERR will show up in the logs. And if you use a log
collector like logspout, you can collect those logs in a central location. See [logging](logging.md).
