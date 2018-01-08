# Go Function Hello World

This example will shows you how to test and deploy Go code to Fn. It will also demonstrate passing JSON data to your function through `stdin`. 

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
1. Run the following command to create a boilerplate Go function: 

>```sh
fn init --runtime go hello
```
    
>A directory named `hello` is created with several files in it.

1. Open the generated `func.go` file and you will see the following source code.

```go
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Person struct {
	Name string
}

func main() {
	p := &Person{Name: "World"}
	json.NewDecoder(os.Stdin).Decode(p)
	mapD := map[string]string{"message": fmt.Sprintf("Hello %s", p.Name)}
	mapB, _ := json.Marshal(mapD)
	fmt.Println(string(mapB))
}
```
<ol start="4">
  <li>The command also generates a <code>yaml</code> metadata file. Open <code>func.yaml</code> file.</li>
</ol>

```yaml
version: 0.0.1
runtime: go
entrypoint: ./func
```

The generated `func.yaml` file contains metadata about your function and declares a number of properties including:

* `version`: Automatically starting at 0.0.1.
* `runtime`: Set automatically based on the presence of `func.go`.
* `entrypoint`: Name of the function to invoke. In this case `./func` which will be the name of the compiled Go file.

These fields are set by default when you run `init` on a function. For more details on [function files go here](../../../../docs/function-file.md).

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

```json
{"message":"Hello World"}
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

```json
{"message":"Hello Johnny"}
```

## Deploy your Function to Fn Server

When you used `fn run` your function was run in your local environment. Now deploy your function to the Fn server we started previously. This server could be running in the cloud, in your datacenter, or on your local machine. In this case we are deploying to our local machine. Enter the following command: 

>```sh
>fn deploy --app goapp --local
>```

The command returns text similar to the following:

```txt
Deploying hello to app: goapp at path: /hello
Bumped to version 0.0.2
Building image noreg/hello:0.0.2 .
Updating route /hello using image noreg/hello:0.0.2...
```

The command creates an app on the server named `goapp`. In addition, a route to your function created based on your directory name: `/hello`. The `--local` option allows the application to deploy without a container registry.

## Test your Function on the Server

With the function deployed to the server, you can make calls to the function. 

### Call your Function without Data

Call your function using the Fn CLI.

>```sh
>fn call goapp /hello
>```

Open a web browser and enter <http://localhost:8080/r/goapp/hello>.

Or try `curl`.
  
>```sh    
>curl http://localhost:8080/r/goapp/hello
>```

All of these options should return:

```json
{"message":"Hello World!"}
```
    
### Call your Function with Data

You can use `curl` to pass JSON data to your function.

>```sh
>curl -X POST -d '{"name":"Johnny"}' -H "Content-Type: application/json" http://localhost:8080/r/goapp/hello
>```

Or specify a file.

>```sh
>curl -X POST -d @payload.json -H "Content-Type: application/json" http://localhost:8080/r/goapp/hello
>```

Both commands should return:

```json
{"message":"Hello Johnny!"}
```

That's it! You have coded your first Go function.

## Learn More

* [Documentation](../../../../docs)
* [Getting Started Series](../../../tutorial)
* [Tutorials](https://github.com/fnproject/tutorials)
