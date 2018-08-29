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

Fn adds `func_name` and `app_name` tags to the container logs using [docker syslog driver tags](https://docs.docker.com/config/containers/logging/syslog/#options):

```
Aug 29 19:41:31 fnserver 178 <11>1 2018-08-29T19:41:31Z 59ff8628a941 app_name=fn-http-func,func_name=/fn-http-func 17 app_name=fn-http-func,func_name=/fn-http-func - 2018/08/29 19:41:31 Example log output
Aug 29 19:41:31 fnserver 154 <14>1 2018-08-29T19:41:31Z 59ff8628a941 app_name=fn-http-func,func_name=/fn-http-func 17 app_name=fn-http-func,func_name=/fn-http-func - HTTP/1.1 200 OK
Aug 29 19:41:31 fnserver 158 <14>1 2018-08-29T19:41:31Z 59ff8628a941 app_name=fn-http-func,func_name=/fn-http-func 17 app_name=fn-http-func,func_name=/fn-http-func - Content-Length: 11
Aug 29 19:41:31 fnserver 139 <14>1 2018-08-29T19:41:31Z 59ff8628a941 app_name=fn-http-func,func_name=/fn-http-func 17 app_name=fn-http-func,func_name=/fn-http-func - 
Aug 29 19:41:31 fnserver 761 <14>1 2018-08-29T19:41:31Z 59ff8628a941 app_name=fn-http-func,func_name=/fn-http-func 17 app_name=fn-http-func,func_name=/fn-http-func - Hello World
```

Currently, Fn does not add `call_id` to the log tags and it's user function responsibility to include this. FDKs will add support for this.
