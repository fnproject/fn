# Introduction

This guide will walk you through creating and testing a simple Lambda function.

We need the the `fn` tool for the rest of this guide. You can install it
by following [these instructions](https://github.com/iron-io/function/fn).

*For this getting started we are assuming you already have working lambda function code available, if not head to the [import instructions] (import.md) and skip the next section.*

## Creating the function

Let's convert the `hello_world` AWS Lambda example to Docker.

```python
def my_handler(event, context):
    message = 'Hello {} {}!'.format(event['first_name'],
                                    event['last_name'])
    return {
        'message' : message
    }
```

Create an empty directory for your project and save this code in a file called
`hello_world.py`.

Now let's use `fn`'s Lambda functionality to create a Docker image. We can
then run the Docker image with a payload to execute the Lambda function.

```sh
$ fn lambda create-function irontest/hello_world:1 python2.7 hello_world.my_handler hello_world.py
Creating directory: irontest/hello_world:1 ... OK
Creating Dockerfile: irontest/hello_world:1/Dockerfile ... OK
Copying file: irontest/hello_world/hello_world:1.py ... OK
Creating function.yaml ... OK
```

As you can see, this is very similar to creating a Lambda function using the
`aws` CLI tool. We name the function as we would name other Docker images. The
`1` indicates the version. You can use any string. This way you can configure
your deployment environment to use different versions. The handler is
the name of the function to run, in the form that python expects
(`module.function`). Where you would package the files into a `.zip` to upload
to Lambda, we just pass the list of files to `fn`.

## Deploying the function to IronFunctions

Next we want to deploy the function to our IronFunctions
```sh
    $ fn deploy -v -d ./irontest irontest
    deploying irontest/hello_world:1/function.yaml
    Sending build context to Docker daemon 4.096 kB
    Step 1 : FROM iron/lambda-python2.7
    latest: Pulling from iron/lambda-python2.7
    c52e3ed763ff: Pull complete
    789cf808491a: Pull complete
    d1b635efed57: Pull complete
    fe23c3dbcfa8: Pull complete
    63c874a9687e: Pull complete
    a6d462dae1df: Pull complete
    Digest: sha256:c5dde3bf3be776c0f6b909d4ad87255a0af9b6696831fbe17c5f659655a0494a
    Status: Downloaded newer image for iron/lambda-python2.7:latest
    ---> 66d3adf47835
    Step 2 : ADD hello_world.py ./hello_world:1.py
    ---> 91a592e0dfa9
    Removing intermediate container 1a1ef40ff0dd
    Step 3 : CMD hello_world.my_handler
    ---> Running in 318da1bba060
    ---> db9b9644168e
    Removing intermediate container 318da1bba060
    Successfully built db9b9644168e
    The push refers to a repository [docker.io/irontest/hello_world:1]
    5d9d142e21b2: Pushed
    11d8145d6038: Layer already exists
    23885f85dbd0: Layer already exists
    6a350a8d14ee: Layer already exists
    e67f7ef625c5: Layer already exists
    321db514ef85: Layer already exists
    6102f0d2ad33: Layer already exists
    latest: digest: sha256:5926ff413f134fa353e4b42f2d4a0d2d4f5b3a39489cfdf6dd5b4a63c4e40dee size: 1784
    updating API with appName: irontest route: /hello_world:1 image: irontest/hello_world:1
    path                                    result
    irontest/hello_world:1/function.yaml     done
```

This will deploy the generated function under the app `irontest` with `hello_world` as a route, e.g:
`http://<hostname>/r/irontest/hello_world:1`,

You should also now see the generated Docker image.

```sh
    $ docker images
    REPOSITORY                TAG         IMAGE ID            CREATED              VIRTUAL SIZE
    irontest/hello_world:1      latest      db9b9644168e        About a minute ago   108.4 MB
    ...
```

## Testing the function

The `test-function` subcommand can launch the Dockerized function with the
right parameters.

```sh
    $ fn lambda test-function irontest/hello_world:1 --payload '{ "first_name": "Jon", "last_name": "Snow" }'
    {"message": "Hello Jon Snow!"}
```

You should see the output.

## Calling the function from IronFunctions

The `fn call` command can call the deployed version with a given payload.

```sh
    $ echo  '{ "first_name": "Jon", "last_name": "Snow" }' | ./fn call irontest /hello_world:1
    {"message": "Hello Jon Snow!"}
```

You should see the output.


## Commands documentation
* [create-function](create.md)
* [test-function](test.md)
* [aws-import](import.md)

## More documentation
* [env](environment.md)
* [aws](aws.md)