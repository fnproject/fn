# Environment Checker Function Image

This images compares the payload info with the header.

## Requirements

- Fn API

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
docker tag $USERNAME/func-checker:latest $USERNAME/func-checker:`cat VERSION`

# pushing to docker hub
docker push $USERNAME/func-checker
```

### Testing image

```
./test.sh
```

## Running it on Fn

### Let's define some environment variables

```
# Set your Function server address
# Eg. 127.0.0.1:8080
FUNCAPI=YOUR_FUNCTIONS_ADDRESS
```

### Running with Fn

With this command we are going to create an application with name `checker`.

```
curl -X POST --data '{
    "app": {
        "name": "checker",
    }
}' http://$FUNCAPI/v1/apps
```

Now, we can create our route

```
curl -X POST --data '{
    "route": {
        "image": "'$USERNAME'/func-checker",
        "path": "/check",
        "config": { "TEST": "1" }
    }
}' http://$FUNCAPI/v1/apps/checker/routes
```

#### Testing function

Now that we created our Fn route, let's test our new route

```
curl -X POST --data '{ "env_vars": { "test": "1" } }' http://$FUNCAPI/r/checker/check
```