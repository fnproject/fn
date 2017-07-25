# Metrics

You can use zipkin to gather stats about the functions server.

Running a zipkin node is easy to get started, they have a docker container:

[zipkin page](http://zipkin.io/pages/quickstart.html)

With zipkin running you can point functions to it using an env var:

`ZIPKIN_URL=http://zipkin:9411/api/v1/spans`

TODO hook up zipkin to poop out to logs/statsd/something else too

## Statsd

The [Logspout Statsd Adapter](https://github.com/treeder/logspout-statsd) adapter can parse the log metrics and forward
them to any statsd server.
