# Blog API Example

A simple serverless blog API

## Requirements

- Remote MongoDB instance (for example heroku)
- Running IronFunctions API

## Development

### Building image locally

```
# SET BELOW TO YOUR DOCKER HUB USERNAME
USERNAME=YOUR_DOCKER_HUB_USERNAME

# build it
docker run --rm -v "$PWD":/go/src/github.com/treeder/hello -w /go/src/github.com/treeder/hello iron/go:dev go build -o function
docker build -t $USERNAME/func-blog .
```

### Publishing to DockerHub

```
# tagging
docker run --rm -v "$PWD":/app treeder/bump patch
docker tag $USERNAME/func-blog:latest $USERNAME/func-blog:`cat VERSION`

# pushing to docker hub
docker push $USERNAME/func-blog
```

## Running it on IronFunctions

First you need a running IronFunctions API

### First, let's define this environment variables

```
# Set your Function server address
# Eg. 127.0.0.1:8080
FUNCAPI=YOUR_FUNCTIONS_ADDRESS

# Set your mongoDB server address
# Eg. 127.0.0.1:27017/blog
MONGODB=YOUR_MONGODB_ADDRESS
```

### Testing image

```
./test.sh
```

### Running with IronFunctions

With this command we are going to create an application with name `blog` and also defining the app configuration `DB`.

```
curl -X POST --data '{
    "app": {
        "name": "blog",
        "config": { "DB": "'$MONGODB'" }
    }
}' http://$FUNCAPI/v1/apps
```

Now, we can create our blog routes:

- `/posts` - to create (authenticated) and list posts
- `/posts/:id` - to read post
- `/token` - to get a JWT

```
curl -X POST --data '{
    "route": {
        "image": "'$USERNAME'/func-blog",
        "path": "/posts"
    }
}' http://$FUNCAPI/v1/apps/blog/routes
```

```
curl -X POST --data '{
    "route": {
        "image": "'$USERNAME'/func-blog",
        "path": "/posts/:id"
    }
}' http://$FUNCAPI/v1/apps/blog/routes
```

```
curl -X POST --data '{
    "route": {
        "image": "'$USERNAME'/func-blog",
        "path": "/token"
    }
}' http://$FUNCAPI/v1/apps/blog/routes
```

#### Testing function

Now that we created our IronFunction route, let's test our routes

```
curl -X POST http://$FUNCAPI/r/blog/posts
```

This command should return `{"error":"Invalid authentication"}` because we aren't sending any token.

#### Authentication

##### Creating a blog user

First let's create our blog user. In this example an user `test` with password `test`.

```
docker run --rm -e DB=$MONGODB -e NEWUSER='{ "username": "test", "password": "test" }' $USERNAME/functions-blog
```

##### Getting authorization token

Now, to get authorized to post in our Blog API endpoints we must request a new token with a valid user.

```
curl -X POST --data '{ "username": "test", "password": "test" }' http://$FUNCAPI/r/blog/token
```

This will output a token like this:

```
{"token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOiIyMDE2LTA5LTAxVDAwOjQzOjMxLjQwNjY5NTIxNy0wMzowMCIsInVzZXIiOiJ0ZXN0In0.aPKdH3QPauutFsFbSdQyF6q1hqTAas_BCbSYi5mFiSU"}
```

Let's save that token in the environment

```
BLOG_TOKEN=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOiIyMDE2LTA5LTAxVDAwOjQzOjMxLjQwNjY5NTIxNy0wMzowMCIsInVzZXIiOiJ0ZXN0In0.aPKdH3QPauutFsFbSdQyF6q1hqTAas_BCbSYi5mFiSU
```

##### Posting in your blog

curl -X POST --header "Authorization: Bearer $BLOG_TOKEN" --data '{ "title": "My New Post", "body": "Hello world!", "user": "test" }' http://$FUNCAPI/r/blog/posts