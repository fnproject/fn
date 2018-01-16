# Packaging your Function

## Option 1 (recommended): Use the `fn` cli tool

We recommend using the [fn cli tool](../fn/README.md) which will handle all of this for you. But if you'd like to dig in
and customize your images, look at Option 2.

## Option 2: Build your own images

Packaging a function has two parts:

* Create a Docker image for your function with an ENTRYPOINT
* Push your Docker image to a registry (Docker Hub by default)

Once it's pushed to a registry, you can use the image location when adding a route.

### Creating an image

The basic Dockerfile for most languages is along these lines:

```
# Choose base image
FROM node:alpine
# Set the working directory
WORKDIR /function
# Add your binary or code to the working directory
ADD . /function/
# Set what will run when a container is started for this image
ENTRYPOINT ["node func.js"]
```

Then build your function image:

```sh
fn build
```

### Push your image

```sh
fn push
```

Now you can use that image when creating or updating routes.