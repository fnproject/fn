# Hybrid API Proposal

TODO fill in auth information herein (possibly do w/o first?)

Hybrid API will consist of a few endpoints that encapsulate all functionality
required for `fn` to run tasks using split API and 'runner' nodes. These
endpoints exist under the `/runner/` endpoints. In addition to these
endpoints, the runner has access to any `/v1/` endpoints it needs as well
(namely, `GetApp` and `GetRoute`).

API nodes are responsible for interacting with an MQ and DB [on behalf of the
runner], as well as handling all requests under the `/v1/` routes.

runner nodes are responsible for receiving requests under the `/r/` endpoints
from the fnlb and sending requests to the `/runner/` endpoints to API nodes,
its duties are:

* enqueueing async calls
* dequeueing async calls when there is spare capacity
* executing calls (both sync and async)
* management of message lifecycle
* reporting call status & logs

## Endpoints

All functionality listed here will be implemented in the API nodes under the
given endpoint. The runner is responsible for calling each of these endpoints
with the given input.

##### /runner/enqueue

this is called when a runner receives a request for an async route.  the
request contains an entire constructed `models.Call` object, as well as an
identifier for this runner node to queue this call to a specific partition in
kafka [mapping to the runner node]***. returns success/fail.

* enqueue an async call to an MQ
* insert a call to the DB with 'queued' state

special cases:

* if enqueue to MQ fails, the request fails and the runner will
reply with a 500 error to the client as if this call never existed
* if insert fails, we ignore this error, which will be handled in Start

##### /runner/dequeue

the runner queries for a call to run. the request contains an identifier for
this runner node to pull from the partition in kafka for this runner node***.
the response contains an app name and a route name (the runner will cache apps
and routes, otherwise looking this up at respective API call positions).

* dequeue a message from the MQ if there is some capacity

##### /runner/start

the runner calls this endpoint immediately before starting a task, only
starting the task if this endpoint returns a success. the request contains the
app name and route name. the response returns success/fail code.

sync:

* noop, the runner will not call this endpoint.

async:

* update call in db to status=running, conditionally. if the status is already
  running, set the status to error as this means that the task has been
  started successfully previously and we don't want to run it twice, and after
  successfully setting the status to error, delete the mq message, and return
  a failure status code. if the status is a final state (error | timeout |
  success), delete the mq message and return a failure status code. if the
  update to status=running succeeds, return a success status code.

##### /runner/finish

the runner calls this endpoint after a call has completed, either because of
an error, a timeout, or because it ran successfully. it will always return a
success code as the call is completed at this point, the runner may retry this
endpoint if it fails (timeout, etc).

sync:

* insert the call model into the db (ignore error, retry)
* insert the log into the log store (ignore error, retry)

async:

* insert the call model into the db (ignore error, retry)
* insert the log into the log store (ignore error, retry)
* delete the MQ message (ignore error, retry, failure is handled in start)

## Additional notes & changes required

* All runner requests and responses will contain a header `XXX-RUNNER-LOAD`
  that API server nodes and FNLB nodes can use to determine how to distribute
  load to that node. This will keep a relatively up to date view of each of
  the runner nodes to the API and FNLB, assuming that each of those are small
  sets of nodes. The API nodes can use this to determine whether to distribute
  messages for async nodes to runner nodes as well as fnlb for routing async
  or sync requests.
* Each runner node will have a partition in Kafka that maps to messages that
  it enqueues. This will allow us to use distribution information, based off
  load, from the load balancer, since the load balancer will send requests to
  queue async tasks optimistically to runner nodes. The runner then only
  processes messages on its partition. This is likely fraught with some
  danger, however, kafka messaging semantics have no idea of timeouts and we
  make no real SLAs about time between enqueue and start, so its somewhat sexy
  to think that runners don't have to think about maneuvering timeouts. This
  likely needs further fleshing out, as noted in***.

`***` current understanding of kafka consumer groups semantics is largely
incomplete and this is making the assumption that if a runner fails, consumer
groups will allow another runner node to cover for this one as well as its
own. distribution should also be considered, and sending in partition ids is
important so that FNLB will dictate the distribution of async functions across
nodes as their load increases, this is also why `XXX-RUNNER-LOAD` is required,
since async tasks aren't returning wait information up stack to the lb. this
likely requires further thought, but is possibly correct as proposed (1% odds)
