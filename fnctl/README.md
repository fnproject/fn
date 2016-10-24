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

## Bulk Update

Also there is the update command that is going to scan all local directory for
functions, rebuild them and push them to Docker Hub and update them in
IronFunction.

```sh
$ fnctl update
Updating for all functions.
path    	    action
/app/hello	    updated
/app/hello-sync	error: no Dockerfile found for this function
/app/test	    updated
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
$ fnctl update
Updating for all functions.
path    	            action
/myapp/route1/subroute1	updated
/other/route1	        updated
```

It means that first subdirectory are always considered app names (e.g. `myapp`
and `other`), each subdirectory of these firsts are considered part of the route
(e.g. `route1/subroute1`).

`fnctl update` expects that each directory to contain a file `functions.yaml`
which instructs `fnctl` on how to act with that particular update, and a
Dockerfile which it is going to use to build the image and push to Docker Hub.

```
$ cat functions.yaml
app: myapp
image: iron/hello
route: "/custom/route"
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

`build` (optional) is an array of shell calls which are used to helping building
the image. These calls are executed before `fnctl` calls `docker build` and
`docker push`.
