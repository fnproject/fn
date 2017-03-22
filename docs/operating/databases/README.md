
# Databases

We currently support the following databases and they are passed in via the `DB_URL` environment variable. For example:

```sh
docker run -e "DB_URL=postgres://user:pass@localhost:6212/mydb" ...
```

## [Bolt](https://github.com/boltdb/bolt) (default)

URL: `bolt:///functions/data/functions.db`

Bolt is an embedded database which stores to disk. If you want to use this, be sure you don't lose the data directory by mounting
the directory on your host. eg: `docker run -v $PWD/data:/functions/data -e DB_URL=bolt:///functions/data/bolt.db ...`

[More on BoltDB](boltdb.md)

## [PostgreSQL](http://www.postgresql.org/)

URL: `postgres://user123:pass456@ec2-117-21-174-214.compute-1.amazonaws.com:6212/db982398`

Use a PostgreSQL database. If you're using IronFunctions in production, you should probably start here.

[More on PostgreSQL](postgres.md)

## [MySQL](https://www.mysql.com/)

URL: `mysql://user123:pass456@tcp(ec2-117-21-174-214.compute-1.amazonaws.com:3306)/funcs`

[More on MySQL](mysql.md)

## What about database X?

We're happy to add more and we love pull requests, so feel free to add one! Copy one of the implementations above as a starting point. 

