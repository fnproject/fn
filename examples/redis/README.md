# Redis GET/SET Function Image

This function basically executes a GET/SET in a given redis server.

## Requirements

- Redis Server
- IronFunctions API

## Development

### Building image locally

```
# SET BELOW TO YOUR DOCKER HUB USERNAME
USERNAME=YOUR_DOCKER_HUB_USERNAME

# build it
./build.sh
```

### Publishing to DockerHub

```
# tagging
docker run --rm -v "$PWD":/app treeder/bump patch
docker tag $USERNAME/func-redis:latest $USERNAME/func-redis:`cat VERSION`

# pushing to docker hub
docker push $USERNAME/func-redis
```

### Testing image

```
./test.sh
```

## Running it on IronFunctions

### Let's define some environment variables

```
# Set your Function server address
# Eg. 127.0.0.1:8080
FUNCAPI=YOUR_FUNCTIONS_ADDRESS

# Set your Redis server address
# Eg. redis:6379
REDIS=YOUR_REDIS_ADDRESS

# Set your Redis server auth (if required)
REDIS_AUTH=REDIS_AUTH_KEY
```

### Running with IronFunctions

With this command we are going to create an application with name `redis`.

```
curl -X POST --data '{
    "app": {
        "name": "redis",
        "config": {
            "server": "'$REDIS'"
            "redis_auth": "'$REDIS_AUTH'"
        }
    }
}' http://$FUNCAPI/v1/apps
```

Now, we can create our routes

#### Route for set value

```
curl -X POST --data '{
    "route": {
        "image": "'$USERNAME'/func-redis",
        "path": "/redis",
        "config": {
            "command": "SET"
        }
    }
}' http://$FUNCAPI/v1/apps/redis/routes
```

#### Route for get value

```
curl -X POST --data '{
    "route": {
        "image": "'$USERNAME'/func-redis",
        "path": "/redis",
        "config": {
            "command": "GET"
        }
    }
}' http://$FUNCAPI/v1/apps/redis/routes
```

#### Testing function

Now that we created our IronFunction route, let's test our new route

```
curl -X POST --data '{"key": "name", "value": "Johnny"}' http://$FUNCAPI/r/redis/set
// "OK"
curl -X POST --data '{"key": "name"}' http://$FUNCAPI/r/redis/get
// "Johnny"
```