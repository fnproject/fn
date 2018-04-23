# CloudEvents - EXPERIMENTAL

Fn supports CloudEvents throughout the system, meaning on ingestion and/or as a function I/O format.

To use as a function I/O format, set `format: cloudevent`.

To use as as the body of the HTTP request, the following header:

```
FN_CLOUD_EVENT: true
```

If that header is set, it is assumed that the function also supports the CloudEvents format (in other words, it will automatically set `format: cloudevent`).

If you have a function that supports CloudEvents, you can test it with the example file in this directory:

```sh
curl -X POST -H "Content-Type: application/json" -H "FN_CLOUD_EVENT: true" -d @ce-example.json http://localhost:8080/r/rapp/myfunc
```

To make a function that supports CloudEvents, you can use an FDK that supports like fdk-ruby.
