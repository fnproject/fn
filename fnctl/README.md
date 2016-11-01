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

## Build and Bump

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

`fnctl bump` will scan all IronFunctions for files named `VERSION` and bump
their version according to [semver](http://semver.org/) rules. In their absence,
it will skip.

