Note: currently running at: http://gateway.iron.computer:8080/

# MicroServices Gateway / API Gateway

First things first, create an app/service:

TOOD: App or service??

```sh
iron create app
# OR
curl -H "Content-Type: application/json" -X POST -d '{"name":"myapp"}' http://localhost:8080/api/v1/apps
```

Now add routes to the app. First we'll add a route to the output of a docker container:

```sh
iron add route myapp /hello iron/hello
# OR
curl -H "Content-Type: application/json" -X POST -d '{"path":"/hello", "image":"iron/hello", "type":"run"}' http://localhost:8080/api/v1/apps/myapp/routes
```

And how about a [slackbot](https://github.com/treeder/slackbots/tree/master/guppy) endpoint:

```sh
curl -H "Content-Type: application/json" -X POST -d '{"path":"/guppy","image":"treeder/guppy:0.0.2", "content_type": "application/json"}' http://localhost:8080/api/v1/apps/myapp/routes
```

Test out the route:

Surf to: http://localhost:8080/hello?app=myapp

You'all also get a custom URL like this when in production.

```
myapp.ironfunctions.com/myroute
```

## Updating Your Images

Tag your images with a version, eg `treeder/guppy:0.0.5` then use that including the tag and update
the route.
