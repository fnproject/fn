
This is just a simple prototype. To get to production would need:

- Routing table in central storage (mongo or IronCache) so all routers can write to it and read to get updates.
- Update routing table from central store every X minutes.
- Remove failed routes and start new workers if it failed. 
- Ability to start new workers if none are running. 
- Ability to always keep a minimum number running at all times, like at least one (or not if on free account?).
- Ability to start new workers based on some auto scaling scheme. 
- Authentication (same as always).


## Testing

- start helloserver.go
- start router.go
- ruby worker.rb a couple times
- ruby client.rb

What's going on?

- worker.rb connects to router and adds routes.
- client.rb connects to router which checks the routing table, proxies the request to one of the destinations and returns the response. 

The idea here is that IronWorker backend can tell the router that it started a process and to start routing requests. The endpoint should only be cached for 55 minutes or so. 

