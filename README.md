# IronFunctions

## Run functions

```sh
docker run --rm --name functions --privileged -it -v $PWD/data:/app/data -p 8080:8080 iron/functions
```

*<b>Note</b>: A list of configurations via env variables can be found [here](docs/api.md).*

## Using Functions

#### Create an Application

An application is essentially a grouping of functions, that put together, form an API. Here's how to create an app. 

```sh
curl -H "Content-Type: application/json" -X POST -d '{
    "app": { "name":"myapp" }
}' http://localhost:8080/v1/apps
```

Now that we have an app, we can map routes to functions. 

#### Add a route to a Function

```sh
curl -H "Content-Type: application/json" -X POST -d '{
    "route": {
        "path":"/hello",
        "image":"iron/hello"
    }
}' http://localhost:8080/v1/apps/myapp/routes
```

#### Calling your Function

Just hit the URL you got back from adding a route above:

```
curl http://localhost:8080/r/myapp/hello
```

#### To pass in data to your function

Your function will get the body of the request as is, and the headers of the request will be passed in as env vars. Try this:

```sh
curl -H "Content-Type: application/json" -X POST -d '{
    "name":"Johnny"
}' http://localhost:8080/r/myapp/hello
```


**Adding a route with URL params**

You can create a route with dynamic URL parameters that will be available inside your function by prefixing path segments with a `:`, for example:

```sh
$ curl -H "Content-Type: application/json" -X POST -d '{
     "route": {
         "path":"/comments/:author_id/:num_page",
         "image":"IMAGE_NAME"
     }
}' http://localhost:8080/v1/apps/myapp/routes
```

`:author_id` and `:num_page` in the path will be passed into your function as `PARAM_AUTHOR_ID` and `PARAM_NUM_PAGE`.


See the [Blog Example](https://github.com/iron-io/functions/blob/master/examples/blog/README.md#creating-our-blog-application-in-your-ironfunctions).


## Adding Asynchronous Data Processing Support

Data processing is for functions that run in the background. This type of functionality is good for functions
that are CPU heavy or take more than a few seconds to complete. 
Architecturally, the main difference between synchronous you tried above and asynchronous is that requests
to asynchronous functions are put in a queue and executed on upon resource availablitiy on the same process
or a remote functions process so that they do not interfere with the fast synchronous responses required by an API.
Also, since it uses a queue, you can queue up millions of jobs without worrying about capacity as requests will
just be queued up and run at some point in the future.  

TODO: Add link to differences here in README.io docs here. 

#### Running remote functions process

Coming soon...

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
