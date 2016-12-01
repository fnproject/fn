## Quick Example for a PHP Function (4 minutes)

This example will show you how to test and deploy Go (Golang) code to IronFunctions.

### 1. Prepare the `func.yaml` file:

At func.yaml you will find:

```yml
name: USERNAME/hello
version: 0.0.1
path: /hello
build:
- docker run --rm -v "$PWD":/worker -w /worker iron/php:dev composer install
```

The important step here is to ensure you replace `USERNAME` with your Docker Hub account name. Some points of note:
the application name is `phpapp` and the route for incoming requests is `/hello`. These informations are relevant for
the moment you try to test this function.

### 2. Build:

```sh
# build the function
fn build
# test it
cat hello.payload.json | fn run
# push it to Docker Hub
fn push
# Create a route to this function on IronFunctions
fn routes create phpapp /hello
```

`-v` is optional, but it allows you to see how this function is being built.

### 3. Queue jobs for your function

Now you can start jobs on your function. Let's quickly queue up a job to try it out.

```sh
cat hello.payload.json | fn call phpapp /hello
```

Here's a curl example to show how easy it is to do in any language:

```sh
curl -H "Content-Type: application/json" -X POST -d @hello.payload.json http://localhost:8080/r/phpapp/hello
```