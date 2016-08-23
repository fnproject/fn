# IronFunctions

## Quick Start


### Start the IronFunctions API

First let's start our IronFunctions API

```sh
docker run --rm --privileged -it -e "DB=bolt:///app/data/bolt.db" -v $PWD/data:/app/data -p 8080:8080 iron/functions
```

This command will quickly start IronFunctions using the default database `Bolt` running on `:8080`.

### Create an Application

An application is essentially a grouping of functions, that put together, form an API. Here's how to create an app. 

```sh
curl -H "Content-Type: application/json" -X POST -d '{
    "app": { "name":"myapp" }
}' http://localhost:8080/v1/apps
```

Now that we have an app, we can add routes to functions. 

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

## Using IronFunctions Hosted by Iron.io

Simply point to https://functions.iron.io instead of localhost and add your Iron.io Authentication header (TODO: link), like this:

```sh
curl -H "Authorization: Bearer IRON_TOKEN" -H "Content-Type: application/json" -X POST -d '{"app": {"name":"myapp"}}' https://functions.iron.io/v1/apps
```

And you'll get an ironfunctions.com host for your app:

```sh
myapp.USER_ID.ironfunctions.com/hello
```

## Full Documentation

http://docs-new.iron.io/docs

## Join Our Community

[![Gitter](https://badges.gitter.im/iron-io/functions.svg)](https://gitter.im/iron-io/functions?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge)
