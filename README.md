# IronFunctions

## [Overview](/iron-io/functions/blob/master/OVERVIEW.md)

## Quick Start

First let's start our IronFunctions API

```
docker run --rm -it -p 8080:8080 iron/functions
```

This command will quickly start our API using the default database `Bolt` running on `:8080`

## Usage

### Creating a application

```sh
curl -H "Content-Type: application/json" -X POST -d '{
    "name":"APP_NAME"
}' http://localhost:8080/v1/apps
```

### Create a route for your Function

Now add routes to the app. First we'll add a route to the output of a docker container:

```sh
curl -H "Content-Type: application/json" -X POST -d '{
    "name": "hello",
    "path":"/hello",
    "image":"iron/hello"
}' http://localhost:8080/v1/apps/myapp/routes
```

### Calling your Function

```
curl http://localhost:8080/r/myapp/hello
```

### To pass in data to your function,

Your function will get the body of the request as is, and the headers of the request will be passed in as env vars. 

```sh
curl -H "Content-Type: application/json" -X POST -d '{
    "name":"Johnny"
}' http://localhost:8080/r/myapp/hello
```

### Using IronFunctions Hosted by Iron.io

Simply point to https://functions.iron.io instead of localhost and add your Iron.io Authentication header (TODO: link), like this:

```sh
curl -H "Authorization: Bearer IRON_TOKEN" -H "Content-Type: application/json" -X POST -d '{"name":"APP_NAME"}' https://functions.iron.io/v1/apps
```

And you'll get an ironfunctions.com host:

```
APP_NAME.USER_ID.ironfunctions.com/PATH
``` 

## Configuring your API

### Databases

These are the current databases supported by IronFunctions:

- [Running with BoltDB](/iron-io/functions/blob/master/docs/database/boltdb.md)
- [Running with Postgres](/iron-io/functions/blob/master/docs/database/postgres.md)

## [Examples](/iron-io/functions/blob/master/examples)

## Logging

TODO

## Monitoring

TODO

## Scaling

TODO

