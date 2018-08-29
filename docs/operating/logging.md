# Logging

There are a few things to note about what Fn logs.

## Logspout

We recommend using [logspout](https://github.com/gliderlabs/logspout) to forward your logs to a log aggregator of your choice.

## Format

All logs are emitted in [logfmt](https://godoc.org/github.com/kr/logfmt) format for easy parsing.

## Call ID

Every function call/request is assigned a `call_id`. If you search your logs, you can track all the activity
for each function call and find errors on a call by call basis.

## Remote syslog for functions

Fn uses [docker syslog driver](https://docs.docker.com/config/containers/logging/syslog/) for remote logging if syslog URL is defined for the application. An example syslog URL is:

```
tcp+tls://logs.papertrailapp.com:1
```

See docker syslog driver [syslog-address](https://docs.docker.com/config/containers/logging/syslog/#options) for supported formats.

In addition to `{{.ID}}` (first 12 characters of the container ID), Fn adds `func_name` and `app_name` tags to the container logs using [docker syslog driver tags](https://docs.docker.com/config/containers/logging/syslog/#options):

```
Aug 28 00:46:19 fnserver func_name=/fn-http-func,app_name=fn-http-func,e97dd9aff479[17]: 2018/08/28 00:46:19 Example log output
Aug 28 00:46:19 fnserver func_name=/fn-http-func,app_name=fn-http-func,e97dd9aff479[17]: HTTP/1.1 200 OK
Aug 28 00:46:19 fnserver func_name=/fn-http-func,app_name=fn-http-func,e97dd9aff479[17]: Content-Length: 11
Aug 28 00:46:19 fnserver func_name=/fn-http-func,app_name=fn-http-func,e97dd9aff479[17]: 
Aug 28 00:46:19 fnserver func_name=/fn-http-func,app_name=fn-http-func,e97dd9aff479[17]: Hello World
```


