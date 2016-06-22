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

Now try mapping an app endpoint:

```sh
curl -H "Content-Type: application/json" -X POST -d '{"path":"/sinatra","image":"treeder/hello-sinatra", "type":"app", "cpath":"/"}' http://localhost:8080/api/v1/apps/myapp/routes
```

And test it out:

```sh
curl -i -X GET http://localhost:8080/sinatra?app=myapp
```

And another:

```sh
curl -H "Content-Type: application/json" -X POST -d '{"path":"/sinatra/ping","image":"treeder/hello-sinatra", "type":"app", "cpath":"/ping"}' http://localhost:8080/api/v1/apps/myapp/routes
```

And test it out:

```sh
curl -i -X GET http://localhost:8080/sinatra?app=myapp
```

You'all also get a custom URL like this when in production.

```
appname.iron.computer
```

## Updating Your Images

Tag your images with a version, eg `treeder/guppy:0.0.5` then use that including the tag and update
the route.

## Building/Testing

Build:

```sh
# one time:
glide install
# then every time
./build.sh
```

Test it, the iron token and project id are for cache.

```sh
docker run -e "IRON_TOKEN=GP8cqlKSrcpmqeR8x9WKD4qSAss" -e "IRON_PROJECT_ID=4fd2729368a0197d1102056b" -e "CLOUDFLARE_EMAIL=treeder@gmail.com" -e "CLOUDFLARE_API_KEY=X" --rm -it --privileged -p 8080:8080 iron/gateway
```

Push it:

```sh
docker push iron/gateway
```

Get it on a server and point router.iron.computer (on cloudflare) to the machine.

After deploying, running it with:

```sh
docker run -e "IRON_TOKEN=GP8cqlKSrcpmqeR8x9WKD4qSAss" -e "IRON_PROJECT_ID=4fd2729368a0197d1102056b" --name irongateway -it --privileged --net=host -p 8080:8080 -d --name irongateway iron/gateway
```

## TODOS

* [ ] Check if image exists when registering the endpoint, not at run time
* [ ] Put stats into influxdb or something to show to user (requests, errors). Or maybe try Keen's new Native Analytics??  Could be faster and easier route. 
* [ ] Store recent logs. Get logs from STDERR, STDOUT is the response.  
* [ ] Allow env vars for config on the app and routes (routes override apps). 
* [ ] Provide a base url for each app, eg: appname.userid.iron.computer
* [ ] Allow setting content-type on a route, then use that when responding
