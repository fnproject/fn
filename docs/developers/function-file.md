# Function files

Functions files are used to assist fn to help you when creating functions.

An example of a function file:

```yaml
name: fnproject/hello
version: 0.0.1
type: sync
memory: 128
config:
  key: value
  key2: value2
  keyN: valueN
headers:
  Content-Type:
  - text/html
build_image: x/y
run_image: x/y
build:
- make
- make test
expects:
  config:
    - name: SECRET_1
      required: true
    - name: SECRET_2
      required: false
```

`name` is the name and tag to which this function will be pushed to and the
route updated to use it.

`path` (optional) allows you to overwrite the calculated route from the path
position. You may use it to override the calculated route. If you plan to use
`fn test --remote=""`, this is mandatory.

`version` represents current version of the function. When deploying, it is
appended to the image as a tag.

`runtime` represents programming language runtime, for example,
'go', 'python', 'java', etc.  The runtime 'docker' will use the existing Dockerfile if one exists.


`type` (optional) allows you to set the type of the route. `sync`, for functions
whose response are sent back to the requester; or `async`, for functions that
are started and return a call ID to customer while it executes in background.
Default: `sync`.

`memory` (optional) allows you to set a maximum memory threshold for this
function. If this function exceeds this limit during execution, it is stopped
and error message is logged. Default: `128`.

`cpus` (optional) is the amount of available CPU cores for this function. For example, `100m` or `0.1`
 +will allow the function to consume at most 1/10 of a CPU core on the running machine. It
 +expects to be a string in MilliCPUs format ('100m') or floating-point number ('0.5').
 +Default: unlimited.

`timeout` (optional) is the maximum time a function will be allowed to run. Default is 30 seconds. 

`headers` (optional) is a set of HTTP headers to be returned in the response of
this function calls.

`config` (optional) is a set of configuration variables to be passed onto the function as environment variables.
These configuration options shall override application configuration during functions execution. See [Configuration](configs.md)
for more information.

`expects` (optional) a list of config/env vars that are required to run this function. These vars will be used when running/testing locally,
if found in your local environment. If these vars are not found, local testing will fail.

`build` (optional) is an array of local shell calls which are used to help
building the function. TODO: Deprecate this?

`build_image` (optional) base Docker image to use for building your function. Default images used are the `dev` tagged images from the [dockers repo](https://github.com/fnproject/dockers).

`run_image` (optional) base Docker image to use for running your function, part of a multi-stage build. Function will be built with `build_image` and run with `run_image`. Default images used from the [dockers repo](https://github.com/fnproject/dockers).

## Hot functions

hot functions support also adds two extra options to this configuration file.

`format` (optional) is one of the streaming formats covered at [function-format.md](function-format.md).

`idle_timeout` (optional) is the time in seconds a container will remain alive without receiving any new requests; 
hot functions will stay alive as long as they receive a request in this interval. Default: `30`.
