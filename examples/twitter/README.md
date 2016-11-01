# Twitter Function Image

This function exemplifies an authentication in Twitter API and get latest tweets of an account.

## Requirements

- IronFunctions API
- Configure a [Twitter App](https://apps.twitter.com/) and [configure Customer Access and Access Token](https://dev.twitter.com/oauth/overview/application-owner-access-tokens).

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
docker tag $USERNAME/func-twitter:latest $USERNAME/func-twitter:`cat VERSION`

# pushing to docker hub
docker push $USERNAME/func-twitter
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

CUSTOMER_KEY="XXXXXX"
CUSTOMER_SECRET="XXXXXX"
ACCESS_TOKEN="XXXXXX"
ACCESS_SECRET="XXXXXX"
```

### Running with IronFunctions

With this command we are going to create an application with name `twitter`.

```
curl -X POST --data '{
    "app": {
        "name": "twitter",
        "config": { 
            "CUSTOMER_KEY": "'$CUSTOMER_KEY'",
            "CUSTOMER_SECRET": "'$CUSTOMER_SECRET'", 
            "ACCESS_TOKEN": "'$ACCESS_TOKEN'",
            "ACCESS_SECRET": "'$ACCESS_SECRET'"
        }
    }
}' http://$FUNCAPI/v1/apps
```

Now, we can create our route

```
curl -X POST --data '{
    "route": {
        "image": "'$USERNAME'/func-twitter",
        "path": "/tweets",
    }
}' http://$FUNCAPI/v1/apps/twitter/routes
```

#### Testing function

Now that we created our IronFunction route, let's test our new route

```
curl -X POST --data '{"username": "getiron"}' http://$FUNCAPI/r/twitter/tweets
```