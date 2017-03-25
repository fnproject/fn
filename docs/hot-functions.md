# Hot functions

IronFunctions is built on top of container technologies, for each incoming
workload, it spins a new container, feed it with the payload and sends the
answer back to the caller. You can expect an average start time of 300ms per
container. You may refer to [this blog](https://medium.com/travis-on-docker/the-overhead-of-docker-run-f2f06d47c9f3#.96tj75ugb) post to understand the details better.

In the case you need faster start times for your function, you may use a hot
container instead.

hot functions are started once and kept alive while there is incoming workload.
Thus, it means that once you decide to use a hot function, you must be able to
tell the moment it should reading from standard input to start writing to
standard output.

Currently, IronFunctions implements a HTTP-like protocol to operate hot
containers, but instead of communication through a TCP/IP port, it uses standard
input/output.

## Implementing a hot function

In the [examples directory](https://github.com/iron-io/functions/blob/master/examples/hotfunctions/http/func.go), there is one simple implementation of a hot function
which we are going to get in the details here.

The basic cycle comprises three steps: read standard input up to a previosly
known point, process the work, the write the output to stdout with some
information about when functions daemon should stop reading from stdout.

In the case at hand, we serve a loop, whose first part is plugging stdin to a
HTTP request parser:

```go
r := bufio.NewReader(os.Stdin)
req, err := http.ReadRequest(r)

// ...
} else {
	l, _ := strconv.Atoi(req.Header.Get("Content-Length"))
	p := make([]byte, l)
	r.Read(p)
}
```

Note how `Content-Length` is used to help determinate how far standard input
must be read.

The next step in the cycle is to do some processing:

```go
	//...
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Hello %s\n", p)
	for k, vs := range req.Header {
		fmt.Fprintf(&buf, "ENV: %s %#v\n", k, vs)
	}
	//...
```

And finally, we return the result with a `Content-Length` header, so
IronFunctions daemon would know when to stop reading the gotten response.

```go
res := http.Response{
	Proto:      "HTTP/1.1",
	ProtoMajor: 1,
	ProtoMinor: 1,
	StatusCode: 200,
	Status:     "OK",
}
res.Body = ioutil.NopCloser(&buf)
res.ContentLength = int64(buf.Len())
res.Write(os.Stdout)
```

Rinse and repeat for each incoming workload.


## Deploying a hot function

Once your functions is adapted to be handled as hot function, you must tell
IronFunctions daemon that this function is now ready to be reused across
requests:

```json
{
	"route":{
		"app_name": "myapp",
		"path": "/hot",
		"image": "USERNAME/hchttp",
		"memory": 64,
		"type": "sync",
		"config": null,
		"format": "http",
		"max_concurrency": "1",
		"idle_timeout": 30
	}
}
```

`format` (mandatory) either "default" or "http". If "http", then it is a hot
container.

`max_concurrency` (optional) - the number of simultaneous hot functions for
this functions. This is a per-node configuration option. Default: 1

`idle_timeout` (optional) - idle timeout (in seconds) before function termination.
