# Redis GET/SET Function Example

This function basically executes a GET/SET in a given redis server.

## How it works

If you send this payload:

```json
{
  "redis": "redis:6379",
  "command": "SET",
  "args": ["name", "Johnny"] 
}
```

The function will execute that command on the server and output:

```
OK
```

If you send this GET payload:

```json
{
  "redis": "redis:6379",
  "command": "GET",
  "args": ["name"] 
}
```

The function will execute that command on the server and output:

```
Johnny
```

## Payload structure

```go
{
	redis     string
	redisAuth string
	command   string
	args      []string
}
```

## Development

### Building image locally

```
# SET BELOW TO YOUR DOCKER HUB USERNAME
USERNAME=YOUR_DOCKER_HUB_USERNAME

# build it
docker build -t $USERNAME/func-redis .
```

### Testing it

Let's run a temporary redis server:

```
docker run --rm --name some-redis redis
```

Now let's test it

```
docker run --link 'some-redis:redis' -e 'PAYLOAD={
    "redis": "redis:6379",
    "command": "SET",
    "args": ["test", "123"]
}' $USERNAME/func-redis
```

Should output:

```
OK
```

### Publishing it

```
# tagging
docker run --rm -v "$PWD":/app treeder/bump patch
docker tag $USERNAME/func-redis:latest $USERNAME/func-redis:`cat VERSION`

# pushing to docker hub
docker push $USERNAME/func-redis
```

## Running it on Functions

First, let's define this two ENV variables

```
# Set your Function server address
# Eg. 127.0.0.1:8080
FUNCHOST=YOUR_FUNCTIONS_ADDRESS

# Set your redis server address
# Eg. myredishost.com:6379
REDISHOST=YOUR_REDIS_ADDRESS

# (OPTIONAL) Set your redis server authentication
REDISAUTH=YOUR_REDIS_AUTH
```

### Creating the route inside your function server

After you [start running you Function server](#), we can create our route:

Eg. /redis/do

```
curl -X POST --data '{
    "name": "do",
    "image": "'$USERNAME'/func-redis",
    "path": "/do"
}' http://$FUNCHOST/v1/apps/redis/routes
```

### Running our function

Now that we created our Function route, lets test it.

```
curl -X POST --data '{
    "redis":"'$REDISHOST'",
    "redisAuth":"'$REDISAUTH'",
    "command": "SET",
    "args":["abc", "123"]
}' http://$FUNCHOST/redis/exec
```