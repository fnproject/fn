# Message Queues

A message queue is used to coordinate asynchronous function calls that run through Fn.

We currently support the following message queues and they are passed in via the `FN_MQ_URL` environment variable. For example:

```sh
docker run -e "FN_MQ_URL=redis://localhost:6379/" ...
```

## [Bolt](https://github.com/boltdb/bolt) (default)

URL: `bolt:///fn/data/functions-mq.db`

See Bolt in databases above. The Bolt database is locked at the file level, so
the file cannot be the same as the one used for the Bolt Datastore.

## [Redis](http://redis.io/)

See Redis in databases above.

## What about message queue X?

We're happy to add more and we love pull requests, so feel free to add one! Copy one of the implementations above as a starting point.
