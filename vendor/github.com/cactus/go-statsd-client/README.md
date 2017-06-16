go-statsd-client
================

[![Build Status](https://travis-ci.org/cactus/go-statsd-client.png?branch=master)](https://travis-ci.org/cactus/go-statsd-client)
[![GoDoc](https://godoc.org/github.com/cactus/go-statsd-client/statsd?status.png)](https://godoc.org/github.com/cactus/go-statsd-client/statsd)
[![Go Report Card](https://goreportcard.com/badge/cactus/go-statsd-client)](https://goreportcard.com/report/cactus/go-statsd-client)


## About

A [StatsD][1] client for Go.

## Docs

Viewable online at [godoc.org][2].

## Example

``` go
import (
    "log"

    "github.com/cactus/go-statsd-client/statsd"
)

func main() {
    // first create a client
    // The basic client sends one stat per packet (for compatibility).
    client, err := statsd.NewClient("127.0.0.1:8125", "test-client")

    // A buffered client, which sends multiple stats in one packet, is
    // recommended when your server supports it (better performance).
    // client, err := statsd.NewBufferedClient("127.0.0.1:8125", "test-client", 300*time.Millisecond, 0)

    // handle any errors
    if err != nil {
        log.Fatal(err)
    }
    // make sure to clean up
    defer client.Close()

    // Send a stat
    client.Inc("stat1", 42, 1.0)
}
```

See [docs][2] for more info. There is also some simple example code in the
`test-client` directory.

## Contributors

See [here][4].

## Alternative Implementations

See the [statsd wiki][5] for some additional client implementations
(scroll down to the Go section).

## License

Released under the [MIT license][3]. See `LICENSE.md` file for details.


[1]: https://github.com/etsy/statsd
[2]: http://godoc.org/github.com/cactus/go-statsd-client/statsd
[3]: http://www.opensource.org/licenses/mit-license.php
[4]: https://github.com/cactus/go-statsd-client/graphs/contributors
[5]: https://github.com/etsy/statsd/wiki#client-implementations
