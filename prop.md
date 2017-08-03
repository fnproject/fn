# Operation: FN Fix It

### A brief intro to the nightmare

Currently, a brief synopsis of user facing woes includes (but is far from
limited to):

* Async functions may never run, and trivially reproducible (queue up 1000
  calls at once).
* Async functions cannot be hot.
* Hot sync functions may timeout even when they could be started on time on
  same host.
* Cold sync functions may timeout even though they ran to completion (and may
  not even respond in the timeout window even if network is 100%).

Concretely:

We have a mix of hot & cold, sync & async, http & plaintext and each one of
those things has its own execution path that contains its own unique bugs.
This seems like a layering issue, mainly.  It would be nice if most of the
code went through the same path, so that we could fix bugs in one place
instead of having to fix them in a few various places to cover, as of now:
hot/async, hot/sync, cold/async.

Inspecting the code will prove that there are pervasive layering issues due to
lack of compartmentalization, and lots of structs that are just different
versions of one another. For example, the http response of a hot or cold sync
function at the moment is a `drivers.RunResult`, which is supposed to be the
output of a container execution, but in this case it is a stand in for a hot
function invocation, as well, which is specifically not the output of a
container execution. Another example is that the `runner` (an object meant to
run docker containers) has a field for a `datastore` (a database), neither of
these two things need the other.

### Proposal

Issues & resolutions, as a list:

##### There are 6 different structs concerned with a 'task' / 'call'

These should all be 1 struct, that implements interfaces that will satisfy
other parts of the system that need to interact with it. For example,
`*models.Task` can implement `drivers.ContainerTask` and `datastore.GetCall`.
Multiple layers could learn to interact with `drivers.ContainerTask`,
alternatively. From here on, 'task' will be referred to as a 'call'.
the '/tasks' endpoints will be herein removed (they currently refer to an
async 'call', with operations to reserve or delete MQ messages directly
through the api. the call API can handle these fine with ID, and look up
the call by id to get fields it needs & remove messages, etc)

##### Execution model for Async is at most once

This needs to be at least once, for starters. We can add exactly once later,
it's just not easy and at least once will work pretty well 99+% of the time for
now. We can also document this as our intended execution model.

##### There is no layer to tie all the things together

We need some kind of layer that will interact with the incoming request
stream, allowing the request stream to peep for slots to execute jobs, and
then actually execute those jobs and store information about them. Right now,
there is just a `runner` that does some things with no clearly defined
boundaries, and sync and async use it differently in their own contexts. On
top of that, `runner` itself has 2 paths for executing calls dependent on
format.

##### Finding a slot for execution is in the driver

Currently, calls will make it down to the runner level before figuring out
that there is no RAM on a host in order to run the container it needs to. At
this point, it waits up to 10s and times out. If a slot is available, then it
will attempt to start the container. Meanwhile, this container could never
start, another 'hot' function could be free to run the call, yet this call
will timeout just waiting on a container that will never come. Async has
similar issues, related to at most once execution, closely.

... I think I could go on for a while, hopefully that's enough to sway.

### Concrete layers:

##### current layer definitions:

http - the layer outlined in `api/server`, which has a datastore, a runner, a
router, an MQ, an enqueue function (lol), a separate db for logs, all the
middleware, a route cache. tl;dr literally jesus

runner - a layer that is responsible for maintaining a bucket of hot
functions, executing jobs, storing the jobs logs, storing the call itself,
allocating slots of ram, pulling async jobs, managing async jobs

datastore - store things in sql (actually good)

MQ - store things in mq (actually ok, these all have different semantics & we
need to play with timeouts)

driver - run docker containers (actually yay)

task / containertask / call / config / task.Request - exposing the same fields
as one another and causing bugs doing so, mostly

##### proposed layer definitions:

http - a router, a datastore and an agent (yes, seriously, the middleware
should be part of the router) -- maybe the MQ, maybe

agent - controller, runner, MQ

controller (name?) - a datastore, a log store, an MQ, a runner

runner - driver, hot function bucket

datastore - store things in sql (actually good)

##### proposed interfaces:

```go
type Agent interface {
  Submit(call) result
}

type Controller interface {
  Start(call)
  End(call)
}

type Runner interface {
  GetSlot(call) Slot
}

type Slot interface {
  Exec() result
}
```

The 'http' layer will only interact with the agent by submitting jobs.  it may
also interact directly with the db or mq for reading / updating / deleting
app / route / call information.

'agent' will have a thread checking the MQ to submit jobs, if ample space.
'agent' will also handle management of both sync and async job execution
(specifically, async will not have its own execution path as like now)

'controller' will wrap up the different semantics of sync and async call
management wrt the data layer (i.e. storing info about the call, not concerned
with the execution of it).  For example, when completing an async job, the
db needs to be updated and the mq needs to be updated; for a sync job, only
the db needs to be updated. These can be implemented by having different
'controller's for each job type,  but maintaining the same execution path in
the 'agent'.

for 'runner', GetSlot will do one of:

* create a container (cold)
* return a slot to a running container (hot) -- may also start one

(this is opposed to previously, where a hot slot would pull a job even while
it is starting a container, this will allow the job to be slotted to another
container while the container it's responsible for starting may be being
started)

##### specific job execution behavior outline

Tasks can enter the system in two ways, by being read off a queue (async) or
from an incoming request (sync).

For sync, if there is no where to run the call, we need to return a 504 that
the server is busy (expressly different than a timeout in the sense that the
call ran, but past its timeout) so that it may run elsewhere.

For cold, pull the image if needed, create the container, start the container

For hot, pull the image if needed, if there are no containers, start a container,
if there are containers, attempt sending the call to one, and if they seem
busy try to start another (since we have the image, this should be ~fast).
