# CloudEvents FDK support  - EXPERIMENTAL

Fn supports CloudEvents throughout the system, meaning on ingestion and/or as a function I/O format.

To use as a function I/O format, set `format: cloudevent`.

And to pass in a full cloud event to a function endpoint, you need to set the Content-Type to `application/cloudevents+json`.

If that header is set, it is assumed that the function also supports the CloudEvents format (in other words, it will automatically set `format: cloudevent`).

## Trying it out

The Ruby FDK supports CloudEvents, so we'll use that:

```sh
fn init --runtime ruby rfunc
cd rfunc
```

Now edit the func.yaml file and change the format to `format: cloudevent`.

Then deploy it:

```
fn deploy --app myapp
```

There's an example cloudevent in this directory so you can test it with this:

```sh
curl -X POST -H "Content-Type: application/cloudevents+json" -d @ce-example.json http://localhost:8080/r/myapp/rfunc
```
