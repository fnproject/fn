

## What's going on?

- worker.rb connects to router and adds routes.
- client.rb connects to router which checks the routing table, proxies the request to one of the destinations and returns the response.

The idea here is that IronWorker backend can tell the router that it started a process and to start routing requests.

## Todo

This is just a simple prototype. To get to production would need:

- Routing table in central storage (mongo or IronCache) so all routers can write to it and read to get updates.
- Update routing table from central store every X minutes.
- Remove failed routes and start new workers if it failed.
- Expire routes after 55 minutes or so.
- Ability to start new workers if none are running. 
- Ability to always keep a minimum number running at all times, like at least one (or not if on free account?).
- Ability to start new workers based on some auto scaling scheme. 
- Authentication (same as always).

## Testing

- start helloserver.go
- start router.go
- ruby worker.rb a couple times
- ruby client.rb

## Testing for reals

- start router.go on remote server (there's a test project on SD already: http://www.simpledeployer.com/projects/ea129e74-52fa-11e2-a91a-12313d008ea2/servers)
  - go build ./src/router; sudo ./router
- iron_worker upload sinatra
- iron_worker queue sinatra
- ruby client.rb
- BOOM!
