# Function logs

We currently support the following function logs stores and they are passed in
via the `LOGSTORE_URL` environment variable. For example:

```sh
docker run -e "LOGSTORE_URL=sqlite3:///functions/logs/fn.db" ...
```

settings `LOGSTORE_URL` to `DB_URL` will put logs in the same database as
other data, this is not recommended for production.

## sqlite3 (default)

example URL: `sqlite3:///functions/logs/fn.db`

sqlite3 is an embedded database which stores to disk. If you want to use this, be sure you don't lose the data directory by mounting
the directory on your host. eg: `docker run -v $PWD/data:/functions/data -e LOGSTORE_URL=sqlite3:///functions/data/fn.db ...`

sqlite3 isn't recommended for production environments
