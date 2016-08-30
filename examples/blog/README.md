# Blog API Example

## Requirements

- Remote MongoDB instance (for example heroku)

## Development

### Building image locally

```
# SET BELOW TO YOUR DOCKER HUB USERNAME
USERNAME=YOUR_DOCKER_HUB_USERNAME

# build it
docker build -t $USERNAME/functions-blog .
```

### Publishing it

```
# tagging
docker run --rm -v "$PWD":/app treeder/bump patch
docker tag $USERNAME/functions-blog:latest $USERNAME/functions-blog:`cat VERSION`

# pushing to docker hub
docker push $USERNAME/functions-blog
```

## Running it on IronFunctions

### First, let's define this environment variables

```
# Set your Function server address
# Eg. 127.0.0.1:8080
FUNCAPI=YOUR_FUNCTIONS_ADDRESS

# Set your mongoDB server address
# Eg. 127.0.0.1:27017
MONGODB=YOUR_MONGODB_ADDRESS
```

### Creating our blog application in your IronFunctions

With this command we are going to create an application with name `blog` and also defining the app configuration `DB`.

```
curl -X POST --data '{
    "app": {
        "name": "blog",
        "config": { "DB": "'$MONGODB'" }
    }
}' http://$FUNCAPI/v1/apps
```

Now, we can create our blog routes: `/posts` and `/posts/:id`

```
curl -X POST --data '{
    "route": {
        "image": "'$USERNAME'/functions-blog",
        "path": "/posts"
    }
}' http://$FUNCAPI/v1/apps/blog/routes
```

```
curl -X POST --data '{
    "route": {
        "image": "'$USERNAME'/functions-blog",
        "path": "/posts/:id"
    }
}' http://$FUNCAPI/v1/apps/blog/routes
```

### Testing our Blog via API

Now that we created our IronFunction route, lets test our routes

```
curl -X GET http://$FUNCAPI/r/blog/posts
curl -X GET http://$FUNCAPI/r/blog/posts/123456
```

These commands should return `{"error":"Invalid authentication"}` because we aren't sending any token.

## Authentication

### Creating a blog user

First let's create our blog user.
```

```

###

To get authorized to access our Blog API endpoints we must request a new token with a valid user. 

```

```