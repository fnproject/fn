# Writing Functions

This will give you the basic overview of writing base level functions. You can also use higher level
abstractions that make it easier such as [lambda](lambda/README.md).

Also, for complete examples in various languages, see the [examples directory](/examples).

## Code

The most basic code layout in any language is as follows, this is pseudo code and is not meant to run.

```ruby
# Read and parse from STDIN
body = JSON.parse(STDIN)

# Do something
return_struct = doSomething(body)

# If sync, respond:
STDOUT.write(JSON.generate(return_struct))
# If async, update something:
db.update(return_struct)
```

## Inputs

Inputs are provided through standard input and environment variables. We'll just talk about the default input format here, but you can find others [here](function-format.md).
To read in the function body, just read from STDIN.

You will also have access to a set of environment variables, independent of
the function's format:

* `FN_APP_NAME` - the name of the application that matched this route, eg: `myapp`
* `FN_PATH` - the matched route, eg: `/hello`
* `FN_METHOD` - the HTTP method for the request, eg: `GET` or `POST`
* `FN_FORMAT` - a string representing one of the [function formats](function-format.md), currently either `default` or `http`. Default is `default`.
* `FN_MEMORY` - a number representing the amount of memory available to the call, in MB
* `FN_TYPE` - the type for this call, currently 'sync' or 'async'

Dependent upon the function's format, additional variables that change on a
per invocation basis will be in a certain location.

For `default` format, these will be in environment variables as well:

* `FN_DEADLINE` - RFC3339 time stamp of the expiration (deadline) date of function execution.
* `FN_REQUEST_URL` - the full URL for the request ([parsing example](https://github.com/fnproject/fn/tree/master/examples/tutorial/params))
* `FN_CALL_ID` - a unique ID for each function execution.
* `FN_METHOD` - http method used to invoke this function
* `FN_HEADER_$X` - the HTTP headers that were set for this request. Replace $X with the upper cased name of the header and replace dashes in the header with underscores.
  * `$X` - $X is the header that came in the http request that invoked this function.

For `http` format these will be in http headers:

* `Fn_deadline` - RFC3339 time stamp of the expiration (deadline) date of function execution.
* `Fn_request_url` - the full URL for the request ([parsing example](https://github.com/fnproject/fn/tree/master/examples/tutorial/params))
* `Fn_call_id` - a unique ID for each function execution.
* `Fn_method` - the HTTP method used to invoke
* `$X` - the HTTP headers that were set for this request, exactly as they were sent in the request.

For `json` format, these will be fields in the json object (see
[format](functions-format.md)):

* `call_id`
* `protocol: { "headers": { "$X": [ "$Y" ] } }` where `$X:$Y` is each http
  header exactly as it was sent in the request

Warning: these may change before release.

## Logging

Standard out is where you should write response data for synchronous functions. Standard error
is where you should write for logging, as [it was intended](http://www.jstorimer.com/blogs/workingwithcode/7766119-when-to-use-stderr-instead-of-stdout).

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

## Using Lambda Functions

### Lambda everywhere

Lambda support for Fn enables you to take your AWS Lambda functions and run them
anywhere. You should be able to take your code and run them without any changes.

Creating Lambda functions is not much different than using regular functions, just use
the `lambda-node` runtime.

```sh
fn init --runtime lambda-node --name lambda-node
```

Be sure the filename for your main handler is `func.js`.

TODO: Make Java and Python use the new workflow too.

## Next Steps

* [Packaging your function](packaging.md)
