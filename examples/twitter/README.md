# Twitter Example

This function exemplifies an authentication in Twitter API and get latest tweets of an account.

## Requirements

You need create or configure a [Twitter App](https://apps.twitter.com/) and configure Customer Access and Access Token.

Reference: https://dev.twitter.com/oauth/overview/application-owner-access-tokens


## Development

### Building image locally

```
# SET BELOW TO YOUR DOCKER HUB USERNAME
export USERNAME=YOUR_DOCKER_HUB_USERNAME

# build it
docker build -t $USERNAME/functions-twitter .
```

### Publishing it

```
docker push $USERNAME/functions-twitter
```

## Running it on IronFunctions

You need a running IronFunctions API

### First, let's define this environment variables

Set your Twitter Credentials in environment variables.

```sh
export CUSTOMER_KEY="XXXXXX"
export CUSTOMER_SECRET="XXXXXX"
export ACCESS_TOKEN="XXXXXX"
export ACCESS_SECRET="XXXXXX"
```

### Create the application
```sh
curl -H "Content-Type: application/json" -X POST -d '{
    "app": { 
        "name": "twitter",
        "config": { 
            "CUSTOMER_KEY": "'$CUSTOMER_KEY'",
            "CUSTOMER_SECRET": "'$CUSTOMER_SECRET'", 
            "ACCESS_TOKEN": "'$ACCESS_TOKEN'",
            "ACCESS_SECRET": "'$ACCESS_SECRET'"
        }
    }
}' http://localhost:8080/v1/apps
```

### Add the route
```sh
curl -H "Content-Type: application/json" -X POST -d '{
    "route": {
        "path":"/tweets",
        "image":"'$USERNAME/functions-twitter'"
    }
}' http://localhost:8080/v1/apps/twitter/routes
```


### Calling the function

```sh
# Latests tweets of default account (getiron)
curl http://localhost:8080/r/twitter/tweets

# Latests tweets of specific account
curl -X POST --data '{"username": "getiron"}' http://localhost:8080/r/twitter/tweets

```