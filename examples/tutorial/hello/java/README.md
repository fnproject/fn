# Java Function Hello World

This example shows you how to test and deploy Java code to Fn. It also demonstrates passing text data to your function through `stdin`. 

To learn more about using Java with Fn check out the [Fn Java FDK](https://github.com/fnproject/fdk-java).

## Before you Begin

This tutorial assumes you have installed Docker, Fn server, and Fn CLI.

## Start Fn Server

Start up the Fn server so we can deploy our function.

>```sh
>fn start
>```

The command starts Fn in single server mode using an embedded database and message queue. You can find all the
configuration options [here](../../../../docs/operating/options.md). 

## Create your Function 

1. Change into the directory where you want to create your function.
1. Run the following command to create a boilerplate Java function: 

>```sh
>fn init --runtime java hello
>```
    
>A directory named `hello` is created with several files and directories in it.

<ol start="3">
  <li>Open the generated <code>src/main/java/com/example/fn/HelloFunction.java</code> file and you will see the following source code.</li>
</ol>

```java
package com.example.fn;

public class HelloFunction {

    public String handleRequest(String input) {
        String name = (input == null || input.isEmpty()) ? "world"  : input;

        return "Hello, " + name + "!";
    }

}
```

<ol start="4">
  <li>The command also generates a <code>yaml</code> metadata file. Open the <code>func.yaml</code> file.</li>
</ol>

```yaml
version: 0.0.1
runtime: java
cmd: com.example.fn.HelloFunction::handleRequest
build_image: fnproject/fn-java-fdk-build:jdk9-1.0.56
run_image: fnproject/fn-java-fdk:jdk9-1.0.56
format: http
```

The generated `func.yaml` file contains metadata about your function and declares a number of properties including:

* `version`: Automatically starting at 0.0.1.
* `runtime`: Set to `java` from the command line.
* `cmd`: Name of the method to invoke. In this case `handleRequest` of the `com.example.fn.HelloFunction` class.

These fields are set by default when you run `init` on a function. For more details on [function files go here](../../../../docs/developers/function-file.md).

## Add Fn Registry Environment Variable

Before we start developing we need to set the `FN_REGISTRY` environment variable. Normally, set the value to your Docker Hub username. However, you can work with Fn locally.  Set the `FN_REGISTRY` variable to an invented value: `noreg`.

>```sh
>export FN_REGISTRY=noreg
>```

The value is used to identify your Fn generated Docker images.

## Test your Function

Test your function using the following command.

>```sh
>fn run
>```

Fn runs your function inside a container exactly how it executes on the server. Notice the first time you run the command it takes a number of seconds. Fn builds the Docker image and the Java source code and then executes the identified method.

When execution is complete, the function returns output to `stout`. In this case, `fn run` returns:

```txt
Hello World!
```

To pass data to our function, pass input to `stdin`. You could pass text data to your function like this:

>```sh
>echo "Johnny" | fn run
>```

Or with.

>```sh
>cat payload.txt | fn run
>```

The function reads the text data and returns:

```txt
Hello Johnny
!
```

## Deploy your Function to Fn Server

When you used `fn run` your function was run in your local environment. Now deploy your function to the Fn server we started previously. This server could be running in the cloud, in your datacenter, or on your local machine. In this case we are deploying to our local machine. Enter the following command: 

>```sh
>fn deploy --app javaapp --local
>```

The command returns text similar to the following:

```txt
Deploying hello to app: javaapp at path: /hello
Bumped to version 0.0.2
Building image noreg/hello:0.0.2 .
Updating route /hello using image noreg/hello:0.0.2...
```

The command creates an app on the server named `javaapp`. In addition, a route to your function created based on your directory name: `/hello`. The `--local` option allows the application to deploy without a container registry.

## Test your Function on the Server

With the function deployed to the server, you can make calls to the function. 

### Call your Function without Data

Call your function using the Fn CLI.

>```sh
>fn call javaapp /hello
>```

Open a web browser and enter <http://localhost:8080/r/javaapp/hello>.

Or try `curl`.
  
>```sh    
>curl http://localhost:8080/r/javaapp/hello
>```

All of these options should return:

```txt
Hello World!
```
    
### Call your Function with Data

You can use `curl` to pass text data to your function.

>```sh
>curl -X POST -d "Johnny" -H "Content-Type: text/plain" http://localhost:8080/r/javaapp/hello
>```

Or specify a file.

>```sh
>curl -X POST -d @payload.txt -H "Content-Type: text/plain" http://localhost:8080/r/javaapp/hello
>```

Both commands should return:

```txt
Hello Johnny!
```

That's it! You have coded your first Java function.

## Learn More

* [Documentation](../../../../docs)
* [Getting Started Series](../../../tutorial)
* [Tutorials](https://github.com/fnproject/tutorials)
