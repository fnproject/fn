# Detailed Usage

This is a more detailed explanation of the main commands you'll use in Fn as a developer.

### Create an Application

An application is essentially a grouping of functions, that put together, form an API. Here's how to create an app.

```sh
fn create app myapp
```

Or using a cURL:

```sh
curl -H "Content-Type: application/json" -X POST -d '{
    "app": { "name":"myapp" }
}' http://localhost:8080/v1/apps
```

[More on apps](apps.md).

Now that we have an app, we can route endpoints to functions.

### Add a Route

A route is a way to define a path in your application that maps to a function. In this example, we'll map
`/hello` to a simple `Hello World!` function called `fnproject/hello` which is a function we already made that you
can use -- yes, you can share functions! The source code for this function is in the [examples directory](/examples/hello/go).

```sh
fn create route myapp /hello -i fnproject/hello
```

Or using cURL:

```sh
curl -H "Content-Type: application/json" -X POST -d '{
    "route": {
        "path":"/hello",
        "image":"fnproject/hello"
    }
}' http://localhost:8080/v1/apps/myapp/routes
```

[More on routes](../operating/routes.md).

### Calling your Function

Calling your function is as simple as requesting a URL. Each app has its own namespace and each route mapped to the app.
The app `myapp` that we created above along with the `/hello` route we added would be called via the following
URL: http://localhost:8080/r/myapp/hello

Either surf to it in your browser or use `fn`:

```sh
fn call myapp /hello
```

Or using a cURL:

```sh
curl http://localhost:8080/r/myapp/hello
```

### Passing data into a function

Your function will get the body of the HTTP request via STDIN, and the headers of the request will be passed in
as env vars. You can test a function with the CLI tool:

```sh
echo '{"name":"Johnny"}' | fn call myapp /hello
```

Or using cURL:

```sh
curl -H "Content-Type: application/json" -X POST -d '{
    "name":"Johnny"
}' http://localhost:8080/r/myapp/hello
```

You should see it say `Hello Johnny!` now instead of `Hello World!`.

### Add an asynchronous function

FN supports synchronous function calls like we just tried above, and asynchronous for background processing.

[Asynchronous functions](async.md) are great for tasks that are CPU heavy or take more than a few seconds to complete.
For instance, image processing, video processing, data processing, ETL, etc.
Architecturally, the main difference between synchronous and asynchronous is that requests
to asynchronous functions are put in a queue and executed on upon resource availability so that they do not interfere with the fast synchronous responses required for an API.
Also, since it uses a message queue, you can queue up millions of function calls without worrying about capacity as requests will
just be queued up and run at some point in the future.

To add an asynchronous function, create another route with the `"type":"async"`, for example:

```sh
curl -H "Content-Type: application/json" -X POST -d '{
    "route": {
        "type": "async",
        "path":"/hello-async",
        "image":"fnproject/hello"
    }
}' http://localhost:8080/v1/apps/myapp/routes
```

or set `type: async` in your `func.yaml`.

Now if you request this route:

```sh
curl -H "Content-Type: application/json" -X POST -d '{
    "name":"Johnny"
}' http://localhost:8080/r/myapp/hello-async
```

You will get a `call_id` in the response:

```json
{"call_id":"572415fd-e26e-542b-846f-f1f5870034f2"}
```

If you watch the logs, you will see the function actually runs in the background:

![async log](/docs/assets/async-log.png)

Read more on [logging](../operating/logging.md).
