# Message Queues

A message queue is used to coordinate asynchronous function calls that run through IronFunctions.

We currently support the following message queues and they are passed in via the `MQ_URL` environment variable. For example:

```sh
docker run -e "MQ_URL=redis://localhost:6379/" ...
```

## [Bolt](https://github.com/boltdb/bolt) (default)

URL: `bolt:///titan/data/functions-mq.db`

See Bolt in databases above. The Bolt database is locked at the file level, so
the file cannot be the same as the one used for the Bolt Datastore.

## [Redis](http://redis.io/)

See Redis in databases above.

## [IronMQ](https://www.iron.io/platform/ironmq/)

URL: `ironmq://project_id:token@mq-aws-us-east-1.iron.io/queue_prefix`

IronMQ is a hosted message queue service provided by [Iron.io](http://iron.io). If you're using IronFunctions in production and don't
want to manage a message queue, you should start here.

The IronMQ connector uses HTTPS by default. To use HTTP set the scheme to
`ironmq+http`. You can also use a custom port. An example URL is:
`ironmq+http://project_id:token@localhost:8090/queue_prefix`.

## What about message queue X?

We're happy to add more and we love pull requests, so feel free to add one! Copy one of the implementations above as a starting point.

