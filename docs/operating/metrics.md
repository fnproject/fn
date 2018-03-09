# Metrics

## Zipkin

You can use zipkin to gather traces from fn.

Running a zipkin node is easy to get started, they have a docker container:

[zipkin page](http://zipkin.io/pages/quickstart.html)

With zipkin running you can point functions to it using an env var:

`ZIPKIN_URL=http://zipkin:9411/api/v1/spans`

Open your browser to observe:

`http://localhost:9411`

## Jaeger

We have support for Jaeger traces, as well.

It is easy to get an all-in-one container of jaeger running to test:

[jaeger](http://jaeger.readthedocs.io/en/latest/getting_started/#all-in-one-docker-image)

And then point fn to jaeger with the environment variable:

`JAEGER_URL=http://jaeger:14268`

Open browser to observe:

`http://localhost:16686`

## Prometheus

Fn offers a prometheus metrics endpoint at `/metrics`

TODO we need to consolidate docs around this (3 places).
