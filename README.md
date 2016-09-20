# IronFunctions

## Quick Start

### Start the IronFunctions API

First let's start our IronFunctions API

```sh
docker run --rm --name functions --privileged -it -e "DB=bolt:///app/data/bolt.db" -v $PWD/data:/app/data -p 8080:8080 iron/functions
```

This command will quickly start IronFunctions using an embedded `Bolt` database running on `:8080`. 

### Create an Application

An application is essentially a grouping of functions, that put together, form an API. Here's how to create an app. 

```sh
curl -H "Content-Type: application/json" -X POST -d '{
    "app": { "name":"myapp" }
}' http://localhost:8080/v1/apps
```

Now that we have an app, we can map routes to functions. 

### Add a route to a Function

```sh
curl -H "Content-Type: application/json" -X POST -d '{
    "route": {
        "path":"/hello",
        "image":"iron/hello"
    }
}' http://localhost:8080/v1/apps/myapp/routes
```

### Calling your Function

Just hit the URL you got back from adding a route above:

```
curl http://localhost:8080/r/myapp/hello
```

### To pass in data to your function

Your function will get the body of the request as is, and the headers of the request will be passed in as env vars. Try this:

```sh
curl -H "Content-Type: application/json" -X POST -d '{
    "name":"Johnny"
}' http://localhost:8080/r/myapp/hello
```


Adding a route with URL params:

A route can pass params to a function by URL.

Example: This set the `PARAM_AUTHOR_ID` and `PARAM_NUM_PAGE` environment variables with value of params (`:author_id` and `:num_page`) passed in the URL.

```sh
$ curl -H "Content-Type: application/json" -X POST -d '{
     "route": {
         "path":"/comments/:author_id/:num_page",
         "image":"IMAGE_NAME"
     }
}' http://localhost:8080/v1/apps/myapp/routes
```


See the [Blog Example](https://github.com/iron-io/functions/blob/master/examples/blog/README.md#creating-our-blog-application-in-your-ironfunctions).


## Adding Asynchronous Data Processing Support

Data processing is for functions that run in the background. This type of functionality is good for functions that are CPU heavy or take more than a few seconds to complete. 
Architecturally, the main difference between synchronous you tried above and asynchronous is that requests
to asynchronous functions are put in a queue and executed on separate `runner` machines so that they do not interfere with the fast synchronous responses required by an API. Also, since 
it uses a queue, you can queue up millions of jobs without worrying about capacity as requests will just be queued up and run at some point in the future.  

TODO: Add link to differences here in README.io docs here. 

### Start Runner(s)

Start a runner:

```sh
docker run --rm -it --link functions --privileged -e "API_URL=http://functions:8080" iron/functions-runner
```

You can start as many runners as you want. The more the merrier.

For runner configuration, see the [Runner README](runner/README.md).

## Using IronFunctions Hosted by Iron.io

Simply point to https://functions.iron.io instead of localhost and add your Iron.io Authentication header (TODO: link), like this:

```sh
curl -H "Authorization: Bearer IRON_TOKEN" -H "Content-Type: application/json" -X POST -d '{"app": {"name":"myapp"}}' https://functions.iron.io/v1/apps
```

And you'll get an ironfunctions.com host for your app:

```sh
myapp.USER_ID.ironfunctions.com/hello
```

## API Reference

https://swaggerhub.com/api/iron/functions

## Full Documentation

http://docs-new.iron.io/docs

## Join Our Community

[![Slack Status](https://open-iron.herokuapp.com/badge.svg)](https://open-iron.herokuapp.com)
