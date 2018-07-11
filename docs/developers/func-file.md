# Func files

Func files are used to assist the fn-project to help you when creating functions.

An example of a func file:

```yaml
schema_version: 20180708
name: fnproject/hello
version: 0.0.1
runtime: java
build_image: x/y
run_image: x/y
cmd: com.example.fn.HelloFunction::handleRequest
memory: 128
timeout: 30
config:
  key: value
  key2: value2
  keyN: valueN
build:
- make
- make test
expects:
  config:
    - name: SECRET_1
      required: true
    - name: SECRET_2
      required: false
triggers:
  - name: triggerOne
    type: http
    source: /trigger-path
```
`schema_version` represents the version of the specification for this file.

`name` is the name and tag to which this function will be pushed to and the
route updated to use it.

`version` represents the current version of the function. When deploying, it is appended to the image as a tag.

`runtime` represents programming language runtime, for example,
'go', 'python', 'java', etc.  The runtime 'docker' will use the existing Dockerfile if one exists.

`build_image` (optional) base Docker image to use for building your function. Default images used are the `dev` tagged images from the [dockers repo](https://github.com/fnproject/dockers).

`run_image` (optional) base Docker image to use for running your function, part of a multi-stage build. Function will be built with `build_image` and run with `run_image`. Default images used from the [dockers repo](https://github.com/fnproject/dockers).

`cmd` (optional) execution command for jvm based runtimes

`entrypoint` (optional) excution entry point for native runtimes

`memory` (optional) allows you to set a maximum memory threshold for this
function. If this function exceeds this limit during execution, it is stopped
and error message is logged. Default: `128`.

`timeout` (optional) is the maximum time a function will be allowed to run. Default is 30 seconds.

`config` (optional) is a set of configuration variables to be passed onto the function as environment variables.
These configuration options shall override application configuration during functions execution. See [Configuration](configs.md)
for more information.

`expects` (optional) a list of config/env vars that are required to run this function. These vars will be used when running/testing locally,
if found in your local environment. If these vars are not found, local testing will fail.

`build` (optional) is an array of local shell calls which are used to help
building the function. TODO: Deprecate this?

`triggers` (optional) is an array of `trigger` entities that specific triggers for the function. See [Trigger](triggers.md).
