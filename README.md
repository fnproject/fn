
## What's going on?

- worker.rb connects to router and adds routes.
- client.rb connects to router which checks the routing table, proxies the request to one of the destinations and returns the response.

The idea here is that IronWorker backend can tell the router that it started a process and to start routing requests.

## Todo

This is just a simple prototype. To get to production would need:

- Ability to start new workers based on some auto scaling scheme.
- Authentication (same as always).

## Testing for reals on staging

Using DockerJockey:

`dj run -i --name mygoprog -v "$(pwd)":/app -w /app -p 8080:8080 treeder/golang-ubuntu:1.3.3on14.04 ./mygoprog`

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
