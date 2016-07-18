Note: currently running at: http://gateway.iron.computer:8080/

# IronFunctions

First, let's fire up an IronFunctions instance. Copy the [example.env](example.env) file into a file named `.env` and fill in the missing values. 

Then start your functions instance:

```
docker run --env-file .env --rm -it --privileged -p 8080:8080 iron/functions
```

## Usage

First things first, create an app/service:
TOOD: App or service??

### Create App

```sh
iron create app APP_NAME
# OR
curl -H "Content-Type: application/json" -X POST -d '{"name":"APP_NAME"}' http://localhost:8080/api/v1/apps
```

### Create a Route 

Now add routes to the app. First we'll add a route to the output of a docker container:

```sh
iron add route myapp /hello iron/hello
# OR
curl -H "Content-Type: application/json" -X POST -d '{"path":"/hello", "image":"iron/hello"}' http://localhost:8080/api/v1/apps/myapp/routes
```

Surf to your function: http://localhost:8080/hello?app=APP_NAME . Boom! 

And how about a [slackbot](https://github.com/treeder/slackbots/tree/master/guppy) endpoint:

```sh
curl -H "Content-Type: application/json" -X POST -d '{"path":"/guppy","image":"treeder/guppy:0.0.2", "content_type": "application/json"}' http://localhost:8080/api/v1/apps/myapp/routes
```

### Using IronFunctions Hosted by Iron.io

Simply point to https://functions.iron.io instead of localhost and add your Iron.io Authentication header (TODO: link), like this:

```sh
curl -H "Authorization: Bearer IRON_TOKEN" -H "Content-Type: application/json" -X POST -d '{"name":"APP_NAME"}' https://functions.iron.io/api/v1/apps
```

And you'll get an ironfunctions.com host:

```
APP_NAME.ironfunctions.com/PATH
```

## Updating Your Images

Tag your images with a version, eg `treeder/guppy:0.0.5` then use that including the tag and update
the route.
