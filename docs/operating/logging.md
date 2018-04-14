# Logging

There are a few things to note about what Fn logs.

## Logspout

We recommend using [logspout](https://github.com/gliderlabs/logspout) to forward your logs to a log aggregator of your choice.

## Format

All logs are emitted in [logfmt](https://godoc.org/github.com/kr/logfmt) format for easy parsing.

## Call ID

Every function call/request is assigned a `call_id`. If you search your logs, you can track all the activity
for each function call and find errors on a call by call basis. For example, these are the log lines for an aynschronous
function call:

![async logs](/docs/assets/async-log-full.png)

Note the easily searchable `call_id=x` format.

```sh
call_id=477949e2-922c-5da9-8633-0b2887b79f6e
```

## Remote syslog for functions

You may add a syslog url to any function application and all functions that
exist under that application will ship all of their logs to it. You may
provide a comma separated list, if desired. Currently, we support `tcp`,
`udp`, and `tls`, and this will not work if behind a proxy [yet?] (this is my
life now). This feature only works for 'hot' functions.

An example syslog url is:

```
tls://logs.papertrailapp.com:1
```

We log in a syslog format, with some variables added in logfmt format. If you
find logfmt format offensive, please open an issue and we will consider adding
more formats (or open a PR that does it, with tests, and you will receive 1
free cookie along with the feature you want). The logs from the functions
themselves are not formatted, only our pre-amble, thus, if you'd like a fully
logfmt line, you must use a logfmt logger to log from your function.

* All log lines are sent as level error w/ the current time and `fn` as hostname.
* call_id, func_name, and app_id will prefix every log line.

```
<11>2 1982-06-25T12:00:00Z fn - - - - call_id=12345 func_name=yo/yo app_id=54321 this is your log line
```
