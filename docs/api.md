
## API Options

#### Env Variables

<table>
<tr>
<th>Env Variables</th>
<th>Description</th>
</tr>
<tr>
<td>DB</td>
<td>The database URL to use in URL format. See Databases below for more information. Default: BoltDB in current working directory `bolt.db`.</td>
</tr>
<tr>
<td>PORT</td>
<td>Default (8080), sets the port to run on.</td>
</tr>
<tr>
<td>MQ</td>
<td>The message queue to use in URL format. See Message Queues below for more information. Default: BoltDB in current working directory `queue.db`.</td>
</tr>
<tr>
<td>API_URL</td>
<td>The primary functions api URL to pull tasks from (the address is that of another running functions process).</td>
</tr>
<tr>
<td>NUM_ASYNC</td>
<td>The number of async runners in the functions process (default 1).</td>
</tr>
</table>

## Databases

We currently support the following databases and they are passed in via the `DB` environment variable. For example:

```sh
docker run -v /titan/data:/titan/data -e "DB=postgres://user:pass@localhost:6212/mydb" ...
```

### Memory

URL: `memory:///`

Stores all data in memory. Fast and easy, but you'll lose all your data when it stops! NEVER use this in production.

### [Bolt](https://github.com/boltdb/bolt)

URL: `bolt:///titan/data/bolt.db`

Bolt is an embedded database which stores to disk. If you want to use this, be sure you don't lose the data directory by mounting
the directory on your host. eg: `docker run -v $PWD/data:/titan/data -e DB=bolt:///titan/data/bolt.db ...`

### [Redis](http://redis.io/) 

URL: `redis://localhost:6379/`

Uses any Redis instance. Be sure to enable [peristence](http://redis.io/topics/persistence). 

### [PostgreSQL](http://www.postgresql.org/)

URL: `postgres://user3123:passkja83kd8@ec2-117-21-174-214.compute-1.amazonaws.com:6212/db982398`

If you're using Titan in production, you should probably start here.

### What about database X?

We're happy to add more and we love pull requests, so feel free to add one! Copy one of the implementations above as a starting point. 

## Message Queues

A message queue is used to coordinate the jobs that run through Titan. 

We currently support the following message queues and they are passed in via the `MQ` environment variable. For example:

```sh
docker run -v /titan/data:/titan/data -e "MQ=redis://localhost:6379/" ...
```

### Memory 

See memory in databases above.

### Bolt

URL: `bolt:///titan/data/bolt-mq.db`

See Bolt in databases above. The Bolt database is locked at the file level, so
the file cannot be the same as the one used for the Bolt Datastore.

### Redis

See Redis in databases above.

### What about message queue X?

We're happy to add more and we love pull requests, so feel free to add one! Copy one of the implementations above as a starting point. 

## Troubleshooting

Enable debugging by passing in the `LOG_LEVEL` env var with DEBUG level. 
 

