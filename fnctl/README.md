# IronFunctions CLI

## Build

Ensure you have Go configured and installed in your environment. Once it is
done, run:

```sh
$ make
```

It will build fnctl compatible with your local environment. You can test this
CLI, right away with:

```sh
$ ./fnctl
```

## Basic
You can operate IronFunctions from the command line.

```sh
$ fnctl apps                                       # list apps
myapp

$ fnctl apps create otherapp                       # create new app
otherapp created

$ fnctl apps describe otherapp                     # describe an app
app: otherapp
no specific configuration

$ fnctl apps
myapp
otherapp

$ fnctl routes myapp                               # list routes of an app
path	image
/hello	iron/hello

$ fnctl routes create otherapp /hello iron/hello   # create route
/hello created with iron/hello

$ fnctl routes delete otherapp hello              # delete route
/hello deleted
```

## Changing target host

`fnctl` is configured by default to talk to a locally installed IronFunctions.
You may reconfigure it to talk to a remote installation by updating a local
environment variable (`$API_URL`):
```sh
$ export API_URL="http://myfunctions.example.org/"
$ fnctl ...
```

## Publish

Also there is the publish command that is going to scan all local directory for
functions, rebuild them and push them to Docker Hub and update them in
IronFunction.

```sh
$ fnctl publish
path    	    result
/app/hello	    done
/app/hello-sync	error: no Dockerfile found for this function
/app/test	    done
```

It works by scanning all children directories of the current working directory,
following this convention:

<pre><code>┌───────┐
│  ./   │
└───┬───┘
    │     ┌───────┐
    ├────▶│ myapp │
    │     └───┬───┘
    │         │     ┌───────┐
    │         ├────▶│route1 │
    │         │     └───────┘
    │         │         │     ┌─────────┐
    │         │         ├────▶│subroute1│
    │         │         │     └─────────┘
    │
    │     ┌───────┐
    ├────▶│ other │
    │     └───┬───┘
    │         │     ┌───────┐
    │         ├────▶│route1 │
    │         │     └───────┘</code></pre>


It will render this pattern of updates:

```sh
$ fnctl publish
path    	            result
/myapp/route1/subroute1	done
/other/route1	        done
```

It means that first subdirectory are always considered app names (e.g. `myapp`
and `other`), each subdirectory of these firsts are considered part of the route
(e.g. `route1/subroute1`).

`fnctl update` expects that each directory to contain a file `functions.yaml`
which instructs `fnctl` on how to act with that particular update, and a
Dockerfile which it is going to use to build the image and push to Docker Hub.

## Functions files (functions.yaml)

Functions files are used to assist fnctl to execute bulk updates of your
functions. The files can be named as:

- functions.yaml
- functions.yml
- function.yaml
- function.yml
- functions.json
- function.json
- fn.yaml
- fn.yml
- fn.json

An example of a function file:
```yaml
app: myapp
image: iron/hello
route: "/custom/route"
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

`app` (optional) is the application name to which this function will be pushed
to.

`image` is the name and tag to which this function will be pushed to and the
route updated to use it.

`route` (optional) allows you to overwrite the calculated route from the path
position. You may use it to override the calculated route.

`version` represents current version of the function. When publishing, it is
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
the image. These calls are executed before `fnctl` calls `docker build` and
`docker push`.

## Build, Bump, Push

When dealing with a lot of functions you might find yourself making lots of
individual calls. `fnctl` offers two command to help you with that: `build` and
`bump`.

```sh
$ fnctl build
path    	    result
/app/hello	    done
/app/test	    done
```

`fnctl build` is similar to `publish` except it neither publishes the resulting
docker image to Docker Hub nor updates the routes in IronFunctions server.

```sh
$ fnctl bump
path    	    result
/app/hello	    done
/app/test	    done
```

`fnctl bump` will scan all IronFunctions whose `version` key in function file
follows [semver](http://semver.org/) rules and bump their version according.

`fnctl push` will scan all IronFunctions and push their images to Docker Hub,
and update their routes accordingly.

## Application level configuration

When creating an application, you can configure it to tweak its behavior and its
routes' with an appropriate flag, `config`.

Thus a more complete example of an application creation will look like:
```sh
fnctl apps create --config DB_URL=http://example.org/ otherapp
```

`--config` is a map of values passed to the route runtime in the form of
environment variables prefixed with `CONFIG_`.

Repeated calls to `fnctl apps create` will trigger an update of the given
route, thus you will be able to change any of these attributes later in time
if necessary.

## Route level configuration

When creating a route, you can configure it to tweak its behavior, the possible
choices are: `memory`, `type` and `config`.

Thus a more complete example of route creation will look like:
```sh
fnctl routes create --memory 256 --type async --config DB_URL=http://example.org/ otherapp /hello iron/hello
```

`--memory` is number of usable MiB for this function. If during the execution it
exceeds this maximum threshold, it will halt and return an error in the logs.

`--type` is the type of the function. Either `sync`, in which the client waits
until the request is successfully completed, or `async`, in which the clients
dispatches a new request, gets a task ID back and closes the HTTP connection.

`--config` is a map of values passed to the route runtime in the form of
environment variables prefixed with `CONFIG_`.

Repeated calls to `fnctl route create` will trigger an update of the given
route, thus you will be able to change any of these attributes later in time
if necessary.
