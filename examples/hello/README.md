This is a worker that just echoes the "input" param in the payload.

eg:

This input:

```json
{
  "name": "Johnny Utah"
}
```

Will output:

```
Hello Johnny Utah!
```

## Building Image

```
# SET BELOW TO YOUR DOCKER HUB USERNAME
USERNAME=YOUR_DOCKER_HUB_USERNAME
# build it
docker build -t $USERNAME/hello .
# test it
docker run -e 'PAYLOAD={"name": "Johnny"}' $USERNAME/hello
# tag it
docker run --rm -v "$PWD":/app treeder/bump patch
docker tag $USERNAME/hello:latest $USERNAME/hello:`cat VERSION`
# push it
docker push $USERNAME/hello
```
