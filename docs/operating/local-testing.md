# Testing a running Fn locally

Running functions that call other functions in a local testing environment can
be challenging. This document will outline best practices in functions to make
this work more easily between deployments as well as some tips to help testing
locally.

### Best Practices

Included in each function invocation will be the request url for how the
function was invoked, see [formats](../developers/function-format.md) for where exactly,
this will look something like `request_url` or `FN_REQUEST_URL` depends on the
format. In FDKs, this will be exposed in the context object for each language.
If your function invokes other functions, it's recommended to either configure
the host that your function should use to invoke the other functions yourself
(by sending it in the payload, or elsewhere), or to parse the `request_url`
that Fn includes in a function invocation and use that. This makes it easy to
switch between e.g. `localhost:8080` and `fn.my-company.com`.

### Configuring functions to invoke locally

When running functions that call other functions locally, they will need to be
able to address the fn server to do so. Since functions run in a docker
container, this can be more challenging when using a `localhost` url. One
option is to use something more heavyweight like [ngrok](https://ngrok.com/),
and this will work. It is also possible in a local environment to start `fn`
with the option `FN_DOCKER_NETWORKS=host`, this is equivalent to running each
function container with `--net=host`, where each function can address the
locally running `fn` server to invoke functions against on `localhost`; this
setting is not recommended for production and any behaviors therein should not
be expected in production, it is extremely convenient for testing locally
nonetheless.
