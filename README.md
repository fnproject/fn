
# MicroServices Gateway / API Gateway

First things first, register an app:

```sh
curl -H "Content-Type: application/json" -X POST -d '{"name":"myapp","password":"xyz"}' http://localhost:8080/test/1/projects/123/apps
```

Now add routes to the app. First we'll add a route to the output of a docker container:

```sh
curl -H "Content-Type: application/json" -X POST -d '{"path":"/hello.rb","image":"treeder/hello.rb", "type":"run"}' http://localhost:8080/test/1/projects/123/apps/myapp/routes
```

Now we'll route to the endpoints of an app running in a docker container:

```sh
curl -H "Content-Type: application/json" -X POST -d '{"path":"/hello.rb","image":"treeder/hello.rb", "type":"run"}' http://localhost:8080/test/1/projects/123/apps/myapp/routes
```

Now test out your new routes.
get route:
curl -i -X GET http://localhost:8080/hello.rb?app=myapp

## Building/Testing

```sh
dj go build
dj go run
```



# Previous version:

## What's going on?

- worker.rb connects to router and adds routes.
- client.rb connects to router which checks the routing table, proxies the request to one of the destinations and returns the response.

The idea here is that IronWorker backend can tell the router that it started a process and to start routing requests.

## Usage

```
iron worker upload --name hello-sinatra --host YOURHOST treeder/hello-sinatra
```

Then hit the url:

```
http://router.iron.io/?rhost=YOURHOST
```

## Todo

This is just a simple prototype. To get to production would need:

- Ability to start new workers based on some auto scaling scheme.
- Authentication (same as always).

## Testing for reals on staging

### 1) Deploy router

Using DockerJockey:

```
dj run --on aws -i --name router -v "$(pwd)":/app -w /app -p 80:8080 treeder/golang-ubuntu:1.3.3on14.04 ./router
```

### 2) Update DNS

Update DNS entry `router.iron.io` to point to the newly launched server after dj deploy.


Or SimpleDeployer:

- start router.go on remote server (there's a test project on SD already:
  - http://www.simpledeployer.com/projects/ea129e74-52fa-11e2-a91a-12313d008ea2/servers
  - this is on aws+sandbox@iron.io account
- go build ./src/router; sudo ./router
- iron_worker upload -e staging --project-id 515b657bc731ff1e69000917 --host routertest-staging.irondns.info sinatra
- visit http://routertest.iron.io (or ruby client.rb)
- BOOM!

## Deploying to production

- just deploy as normal from SD project
- use routertest.irondns.info for host
