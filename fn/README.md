# IronFunctions CLI

## Creating Functions

### init

Init will help you create a [function file](../docs/function-file.md) (func.yaml) in the current directory.

To make things simple, we try to use convention over configuration, so `init` will look for a file named `func.{language-extension}`. For example,
if you are using Node, put the code that you want to execute in the file `func.js`. If you are using Python, use `func.py`. Ruby, use `func.rb`. Go, `func.go`. Etc.

Run:

```sh
fn init <DOCKER_HUB_USERNAME>/<FUNCTION_NAME>
```

If you want to override the convention with configuration, you can do that as well using:

```sh
fn init [--runtime node] [--entrypoint "node hello.js"] <DOCKER_HUB_USERNAME>/<FUNCTION_NAME>
```

Or, if you want full control, just make a Dockerfile. If `init` finds a Dockerfile, it will use that instead of runtime and entrypoint.

### Build, Bump, Run, Push

`fn` provides a few commands you'll use while creating and updating your functions: `build`, `bump`, `run` and `push`.

Build will build the image for your function.

```sh
fn build
```

Bump will bump the version number in your func.yaml file. Versions must be in [semver](http://semver.org/) format.

```sh
fn bump
```

Run will help you test your function. Functions read input from STDIN, so you can pipe the payload into the function like this:

```sh
cat `payload.json` | fn run
```

Push will push the function image to Docker Hub.

```sh
fn push
```

## Using the API

You can operate IronFunctions from the command line.

```sh
$ fn apps                                       # list apps
myapp

$ fn apps create otherapp                       # create new app
otherapp created

$ fn apps describe otherapp                     # describe an app
app: otherapp
no specific configuration

$ fn apps
myapp
otherapp

$ fn routes myapp                               # list routes of an app
path	image
/hello	iron/hello

$ fn routes create otherapp /hello iron/hello   # create route
/hello created with iron/hello

$ fn routes delete otherapp hello              # delete route
/hello deleted

$ fn routes headers set otherapp hello header-name value         # add HTTP header to response
otherapp /hello headers updated header-name with value

$ fn version                                   # shows version both of client and server
Client version: 0.1.0
Server version: 0.1.21
```

## Application level configuration

When creating an application, you can configure it to tweak its behavior and its
routes' with an appropriate flag, `config`.

Thus a more complete example of an application creation will look like:
```sh
fn apps create --config DB_URL=http://example.org/ otherapp
```

`--config` is a map of values passed to the route runtime in the form of
environment variables.

Repeated calls to `fn apps create` will trigger an update of the given
route, thus you will be able to change any of these attributes later in time
if necessary.

## Route level configuration

When creating a route, you can configure it to tweak its behavior, the possible
choices are: `memory`, `type` and `config`.

Thus a more complete example of route creation will look like:
```sh
fn routes create --memory 256 --type async --config DB_URL=http://example.org/ otherapp /hello iron/hello
```

`--memory` is number of usable MiB for this function. If during the execution it
exceeds this maximum threshold, it will halt and return an error in the logs.

`--type` is the type of the function. Either `sync`, in which the client waits
until the request is successfully completed, or `async`, in which the clients
dispatches a new request, gets a task ID back and closes the HTTP connection.

`--config` is a map of values passed to the route runtime in the form of
environment variables.

Repeated calls to `fn route create` will trigger an update of the given
route, thus you will be able to change any of these attributes later in time
if necessary.

## Route headers

You can configure a route's HTTP response to return specific headers.

A header configuration workflow example:
```sh
$ fn routes headers set otherapp hello header-name value
otherapp /hello headers updated header-name with value

$ fn routes headers view otherapp hello
otherapp /hello headers:
header-name: [value]

$ fn routes headers unset otherapp hello header-name
otherapp /hello removed header header-name
```

## Changing target host

`fn` is configured by default to talk http://localhost:8080.
You may reconfigure it to talk to a remote installation by updating a local
environment variable (`$API_URL`):
```sh
$ export API_URL="http://myfunctions.example.org/"
$ fn ...
```

## Publish

Also there is the publish command that is going to scan all local directory for
functions, rebuild them and push them to Docker Hub and update them in
IronFunction.

```sh
$ fn publish
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
$ fn publish
path    	            result
/myapp/route1/subroute1	done
/other/route1	        done
```

It means that first subdirectory are always considered app names (e.g. `myapp`
and `other`), each subdirectory of these firsts are considered part of the route
(e.g. `route1/subroute1`).

`fn publish` expects that each directory to contain a file `func.yaml`
which instructs `fn` on how to act with that particular update, and a
Dockerfile which it is going to use to build the image and push to Docker Hub.

## Contributing

Ensure you have Go configured and installed in your environment. Once it is
done, run:

```sh
$ make
```

It will build fn compatible with your local environment. You can test this
CLI, right away with:

```sh
$ ./fn
```
