

# MicroServices Gateway / API Gateway

First things first, register an app:

```sh
curl -H "Content-Type: application/json" -X POST -d '{"name":"myapp","password":"xyz"}' http://localhost:8080/test/1/projects/123/apps
```

Now add routes to the app. First we'll add a route to the output of a docker container:

```sh
curl -H "Content-Type: application/json" -X POST -d '{"path":"/hello.rb","image":"treeder/hello.rb", "type":"run"}' http://localhost:8080/test/1/projects/123/apps/myapp/routes
```

curl -H "Content-Type: application/json" -X POST -d '{"path":"/helloiron","image":"iron/hello", "type":"run"}' http://localhost:8080/test/1/projects/123/apps/myapp/routes

Test out the route:

```sh
curl -i -X GET http://localhost:8080/hello.rb?app=myapp
```

Now try mapping an app endpoint:

```sh
curl -H "Content-Type: application/json" -X POST -d '{"path":"/sinatra","image":"treeder/hello-sinatra", "type":"app", "cpath":"/"}' http://localhost:8080/test/1/projects/123/apps/myapp/routes
```

And test it out:

```sh
curl -i -X GET http://localhost:8080/sinatra?app=myapp
```

And another:

```sh
curl -H "Content-Type: application/json" -X POST -d '{"path":"/sinatra/ping","image":"treeder/hello-sinatra", "type":"app", "cpath":"/ping"}' http://localhost:8080/test/1/projects/123/apps/myapp/routes
```

And test it out:

```sh
curl -i -X GET http://localhost:8080/sinatra?app=myapp
```

You'all also get a custom URL like this when in production.

```
appname.projectid.iron.computer
```

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
docker run -e "IRON_TOKEN=GP8cqlKSrcpmqeR8x9WKD4qSAss" -e "IRON_PROJECT_ID=4fd2729368a0197d1102056b" --rm -it --privileged -p 8080:8080 iron/gateway
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
