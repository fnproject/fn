# Function logs

We currently support the following function logs stores and they are passed in
via the `FN_LOGSTORE_URL` environment variable. For example:

```sh
docker run -e "FN_LOGSTORE_URL=sqlite3:///functions/logs/fn.db" ...
```

settings `FN_LOGSTORE_URL` to `FN_DB_URL` will put logs in the same database as
other data, this is not recommended for production.

## sqlite3 / postgres / mysql (default)

NOTE: if you leave FN_LOGSTORE_URL empty, it will default to FN_DB_URL. this is
recommended if you are not using a separate place for your logs for connection
pooling reasons.

example URL: `sqlite3:///functions/logs/fn.db`

sqlite3 is an embedded database which stores to disk. If you want to use this, be sure you don't lose the data directory by mounting
the directory on your host. eg: `docker run -v $PWD/data:/functions/data -e FN_LOGSTORE_URL=sqlite3:///functions/data/fn.db ...`

sqlite3 isn't recommended for production environments

## minio / s3

If you have an s3-compatible object store, we are using only `put_object` and
`get_object` and you may point `FN_LOGSTORE_URL` at that api's url appropriately.
If you don't have one of those running, you may run minio, an example is
below:

```sh
$ docker run -d -p 9000:9000 --name minio -e "MINIO_ACCESS_KEY=admin" -e "MINIO_SECRET_KEY=password" minio/minio server /data
$ docker run --privileged --link minio -e "FN_LOGSTORE_URL=s3://admin:password@minio:9000/us-east-1/fnlogs" fnproject/fnserver:latest
```

you may include any other necessary args for fnproject, this example only
illustrates running a minio server and the required args for a functions
server to use it.
