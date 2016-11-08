# Packaging your Function

Packaging a function has two parts:

* Create a Docker image for your function with an ENTRYPOINT
* Push your Docker image to a registry (Docker Hub by default)

Once it's pushed to a registry, you can use it by referencing it when adding a route. 

## Creating an image

The basic Dockerfile for most languages is along these lines:

```
# Choose base image
FROM iron/go
# Set the working directory
WORKDIR /function
# Add your binary or code to the working directory
ADD funcbin /function/
# Set what will run when a container is started for this image
ENTRYPOINT ["./funcbin"]
```

Then you simply build your function:

```sh
docker run --rm -v "$PWD":/go/src/$FUNCPKG -w /go/src/$FUNCPKG iron/go:dev go build -o funcbin
docker build -t $USERNAME/myfunction .
```

Or using [fnctl](../fnctl/README.md):

```sh
fnctl build
```

## Push your image

This part is simple:

```sh
docker push $USERNAME/myfunction 
```

Or using [fnctl](../fnctl/README.md):

```sh
fnctl push
```
