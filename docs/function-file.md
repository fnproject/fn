# Function files

Functions files are used to assist fn to help you when creating functions.

The files can be named as:

- func.yaml
- func.json

An example of a function file:

```yaml
name: iron/hello
version: 0.0.1
type: sync
memory: 128
config:
  key: value
  key2: value2
  keyN: valueN
headers:
  content-type:
   - text/plain
build:
- make
- make test
```

`name` is the name and tag to which this function will be pushed to and the
route updated to use it.

`path` (optional) allows you to overwrite the calculated route from the path
position. You may use it to override the calculated route. If you plan to use
`fn test --remote=""`, this is mandatory.

`version` represents current version of the function. When deploying, it is
appended to the image as a tag.

`type` (optional) allows you to set the type of the route. `sync`, for functions
whose response are sent back to the requester; or `async`, for functions that
are started and return a task ID to customer while it executes in background.
Default: `sync`.

`memory` (optional) allows you to set a maximum memory threshold for this
function. If this function exceeds this limit during execution, it is stopped
and error message is logged. Default: `128`.

`headers` (optional) is a set of HTTP headers to be returned in the response of
this function calls.

`config` (optional) is a set of configurations to be passed onto the route
setup. These configuration options shall override application configuration
during functions execution.

`build` (optional) is an array of local shell calls which are used to help
building the function.

## Hot functions

hot functions support also adds two extra options to this configuration file.

`format` (optional) is one of the streaming formats covered at [function-format.md](function-format.md).

`max_concurrency` (optional) is the maximum of hot functions per node to be
started for a certain function. It defaults to one per function. If you
understand you need more processing power, make sure to raise this number.
Keep in mind that if there is not available memory to execute the configured
workload, it will fail to start new hot functions.

## Testing functions

`tests` (optional) is an array of tests that can be used to valid functions both
locally and remotely. It has the following structure

```yaml
tests:
- name: envvar
  in: "inserted stdin"
  out: "expected stdout"
  err: "expected stderr"
  env:
    envvar: trololo
```

`in` (optional) is a string that is going to be sent to the file's declared
function.

`out` (optional) is the expected output for this function test. It is present
both in local and remote executions.

`err` (optional) similar to `out`, however it read from `stderr`. It is only
available for local machine tests.

`env` (optional) is a map of environment variables that are injected during
tests.
