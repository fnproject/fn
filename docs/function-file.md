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
build:
- make
- make test
```

`name` is the name and tag to which this function will be pushed to and the
route updated to use it.

`path` (optional) allows you to overwrite the calculated route from the path
position. You may use it to override the calculated route.

`version` represents current version of the function. When deploying, it is
appended to the image as a tag.

`type` (optional) allows you to set the type of the route. `sync`, for functions
whose response are sent back to the requester; or `async`, for functions that
are started and return a task ID to customer while it executes in background.
Default: `sync`.

`memory` (optional) allows you to set a maximum memory threshold for this
function. If this function exceeds this limit during execution, it is stopped
and error message is logged. Default: `128`.

`config` (optional) is a set of configurations to be passed onto the route
setup. These configuration options shall override application configuration
during functions execution.

`build` (optional) is an array of shell calls which are used to helping building
the image. These calls are executed before `fn` calls `docker build` and
`docker push`.