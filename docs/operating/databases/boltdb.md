# Oracle Functions using BoltDB

SQLite3 is the default database, you just need to run the API.

## Persistent

To keep it persistent, add a volume flag to the command:

```
docker run --rm -it --privileged -v $PWD/fn.db:/app/fn.db -p 8080:8080 treeder/functions
```
