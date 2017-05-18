# Error Function Image

This images compares the payload info with the header.

## Requirements

- Oracle Functions API

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
docker tag $USERNAME/func-error:latest $USERNAME/func-error:`cat VERSION`

# pushing to docker hub
docker push $USERNAME/func-error
```

### Testing image

```
./test.sh
```

## Running it on Oracle Functions

### Let's define some environment variables

```
# Set your Function server address
# Eg. 127.0.0.1:8080
FUNCAPI=YOUR_FUNCTIONS_ADDRESS
```

### Running with Oracle Functions

With this command we are going to create an application with name `error`.

```
curl -X POST --data '{
    "app": {
        "name": "error",
    }
}' http://$FUNCAPI/v1/apps
```

Now, we can create our route

```
curl -X POST --data '{
    "route": {
        "image": "'$USERNAME'/func-error",
        "path": "/error",
    }
}' http://$FUNCAPI/v1/apps/error/routes
```

#### Testing function

Now that we created our Oracle Functions route, let's test our new route

```
curl -X POST --data '{"input": "yoooo"}' http://$FUNCAPI/r/error/error
```