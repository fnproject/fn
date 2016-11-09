# Metrics

Metrics are emitted via the logs for few couple of reasons:

1. Everything supports STDERR.
2. User can optionally use them, if not, they just end up in the logs.
3. No particular metrics system required, in other words, all metrics systems can be used via adapters (see below).

## Metrics

The metrics format follows logfmt format and looks like this:

```
metric=someevent value=1 type=count
metric=somegauge value=50 type=gauge
```

It's a very simple format that can be easily parsed by any logfmt parser and passed on to another stats service.

TODO: List all metrics we emit to logs.

## Statsd

The [Logspout Statsd Adapter](https://github.com/iron-io/logspout-statsd) adapter can parse the log metrics and forward
them to any statsd server.
