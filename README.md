# Oracle Functions

<!-- [![GoDoc](https://godoc.org/github.com/treeder/functions?status.svg)](https://godoc.org/github.com/treeder/functions) -->

Oracle Functions is an open source [serverless](serverless.md) platform, or as we like to refer to it, Functions as a Service (FaaS) platform that you can run anywhere. Some of it's key features:

* Write once
  * [Any language](docs/faq.md#which-languages-are-supported)
  * [AWS Lambda format supported](docs/lambda/README.md)
* [Run anywhere](docs/faq.md#where-can-i-run-functions)
  * Public, private and hybrid cloud
  * [Import functions directly from Lambda](docs/lambda/import.md) and run them wherever you want
* Easy to use [for developers](docs/README.md#for-developers)
* Easy to manage [for operators](docs/README.md#for-operators)
* Written in [Go](https://golang.org)


## Prequisites

* Docker 17.05 or later installed and running
* Logged into Docker Hub (`docker login`)

## Usage

### Installation 

NOTE: The following instructions apply while the project is a private repo. This will 
build the Functions server and the CLI tool directly from the repo instead of
using pre-built containers. Once the project is public, these steps will be unnecessary.

```sh
# Build and Install CLI tool
cd fn
make dep # just once
make install

# Build and Run Functions Server
cd ..
make dep # just once
make run # will build as well
```

<!-- ADD BACK ONCE PUBLIC 

### Install CLI tool

This isn't required, but it sure makes things a lot easier. Just run the following to install:

```sh
curl -LSs https://goo.gl/KKDFGn | sh
```

This will download a shell script and execute it.  If the script asks for a password, that is because it invokes sudo.

### Run Oracle Functions Server

To get started quickly with Oracle Functions, just fire up a functions container:

```sh
fn start
```

This will start Oracle Functions in single server mode, using an embedded database and message queue. You can find all the
configuration options [here](docs/operating/options.md). If you are on Windows, check [here](docs/operating/windows.md).

-->

### Writing Your First Function

Functions are small but powerful blocks of code that generally do one simple thing. Forget about monoliths when using functions, just focus on the task that you want the function to perform.

Start with this readme tutorial, and then you can learn more about function best practices in 
our section [Writing Functions](docs/writing.md).

The following is a simple Go program that outputs a string to STDOUT. Copy and paste the code below into a file called `func.go`.

```go
package main

import (
	"fmt"
)

func main() {
	fmt.Println("Boom. Oracle Functions.")
}
```

Now run the following commands to build your function and deploy it:

```sh
# Create your first application
fn apps create myapp

# Initilizes your function w/ prebuilt func.yaml
# Replace $USERNAME with your DockerHub username
fn init $USERNAME/hello

# Test your function
# This will run inside a container exactly how it will on the server
fn run

# Deploy it to your functions server (default localhost:8080)
# This will create a route to your function as well
fn deploy myapp
```

Boom. Now you can call your function:

```sh
curl http://localhost:8080/r/myapp/hello
```

Or in a browser: [http://localhost:8080/r/myapp/hello](http://localhost:8080/r/myapp/hello)

That's it! You just deployed your first function and called it. Now to update your function 
you can update your code and run ```fn deploy myapp``` again.

## Learning More

### Documentation

See [docs/](docs/README.md) for full documentation.

More on [Writing Functions](docs/writing.md).

And you can find a bunch of examples in the [/examples](/examples) directory.

You can also write your functions in AWS's [Lambda format](docs/lambda/README.md).

### Get Involved

TODO: Slack or Discord community. 

See [contributing](CONTRIBUTING.md).


## Functions UI

```sh
docker run --rm -it --link functions:api -p 4000:4000 -e "API_URL=http://api:8080" treeder/functions-ui
```

For more information, see: https://github.com/treeder/functions-ui


## Roadmap

See [milestones](https://gitlab.oracledx.com/odx/functions/milestones) for detailed issues.


