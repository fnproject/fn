
# Function logs

We currently support the following function logs stores and they are passed in via the `LOGSTORE_URL` environment variable. For example:
Maximum size of single log entry: 4Mb


```sh
docker run -e "LOGSTORE_URL=bolt:///functions/logs/bolt.db" ...
```

## [Bolt](https://github.com/boltdb/bolt) (default)

URL: `bolt:///functions/logs/bolt.db`

Bolt is an embedded database which stores to disk. If you want to use this, be sure you don't lose the data directory by mounting
the directory on your host. eg: `docker run -v $PWD/data:/functions/data -e LOGSTORE_URL=bolt:///functions/data/bolt.db ...`

[More on BoltDB](../databases/boltdb.md)
