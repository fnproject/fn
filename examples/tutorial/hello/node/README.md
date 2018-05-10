# Node Function Hello World

This example shows you how to test and deploy Node code to Fn. It also demonstrates passing JSON data to your function through `stdin`. 

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

* Change into the directory where you want to create your function.
* Create a `func.js` file.
* Add the following source code and save the file.

```js
name = "World";
fs = require('fs');
try {
    obj = JSON.parse(fs.readFileSync('/dev/stdin').toString());
    if (obj.name != "") {
        name = obj.name;
    }
} catch(e) {}
console.log("Hello", name, "from Node!");
```

* Run the following command to create a `func.yaml` configuration file: 

>```sh
>fn init --runtime node
>```

* The command also generates a `func.yaml` metadata file. Open the `func.yaml` file.

```yaml
version: 0.0.1
runtime: node
entrypoint: node func.js
```

The generated `func.yaml` file contains metadata about your function and declares a number of properties including:

* `version`: Automatically starting at 0.0.1.
* `runtime`: Set to `node` from the command line.
* `entrypoint`: The command that runs the function.

These fields are set by default when you run `init` on a function. For more details on [function files go here](https://github.com/fnproject/fn/blob/master/docs/developers/function-file.md).

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

Fn runs your function inside a container exactly how it executes on the server. When execution is complete, the function returns output to `stout`. In this case, `fn run` returns:

```txt
Hello World from Node!
```

To pass data to our function, pass input to `stdin`. You could pass JSON data to your function like this:

>```sh
>echo '{"name":"Johnny"}' | fn run
>```

Or with.

>```sh
>cat payload.json | fn run
>```

The function reads the JSON data and returns:

```txt
Hello Johnny from Node!
```

## Deploy your Function to Fn Server

When you used `fn run` your function was run in your local environment. Now deploy your function to the Fn server we started previously. This server could be running in the cloud, in your datacenter, or on your local machine. In this case we are deploying to our local machine. Enter the following command: 

>```sh
>fn deploy --app nodeapp --local
>```

The command returns text similar to the following:

```txt
Deploying hello to app: nodeapp at path: /hello
Bumped to version 0.0.2
Building image noreg/hello:0.0.2 
Updating route /hello using image noreg/hello:0.0.2...
```

The command creates an app on the server named `nodeapp`. In addition, a route to your function created based on your directory name: `/hello`. The `--local` option allows the application to deploy without a container registry.

## Test your Function on the Server

With the function deployed to the server, you can make calls to the function. 

### Call your Function without Data

Call your function using the Fn CLI.

>```sh
>fn call nodeapp /hello
>```

Open a web browser and enter <http://localhost:8080/r/nodeapp/hello>.

Or try `curl`.
  
>```sh    
>curl http://localhost:8080/r/nodeapp/hello
>```

All of these options should return:

```txt
Hello World from Node!
```
    
### Call your Function with Data

You can use `curl` to pass JSON data to your function.

>```sh
>curl -X POST -d '{"name":"Johnny"}' -H "Content-Type: application/json" http://localhost:8080/r/nodeapp/hello
>```

Or specify a file.

>```sh
>curl -X POST -d @payload.json -H "Content-Type: application/json" http://localhost:8080/r/nodeapp/hello
>```

Both commands should return:

```txt
Hello Johnny from Node!
```

That's it! You have coded your first Node function.

### Note on Dependencies

You can create a <code>[package.json](https://docs.npmjs.com/getting-started/using-a-package.json)</code> file in your functions directory. The command line interface picks that up and builds all your dependencies on `fn run` and `fn deploy`.


## Learn More

* [Documentation](../../../../docs)
* [Getting Started Series](../../../tutorial)
* [Tutorials](https://github.com/fnproject/tutorials)
