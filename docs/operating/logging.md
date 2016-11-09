# Logging

There are a few things to note about what IronFunctions logs.

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

## Metrics

Metrics are emitted via the logs.

See [Metrics](metrics.md) doc for more information.

