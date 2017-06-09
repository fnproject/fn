# Oracle Functions [![build status](https://gitlab-odx.oracle.com/odx/functions/badges/master/build.svg)](https://gitlab-odx.oracle.com/odx/functions/commits/master)

<!-- [![GoDoc](https://godoc.org/github.com/treeder/functions?status.svg)](https://godoc.org/github.com/treeder/functions) -->

Oracle Functions is an event-driven, open source, [functions-as-a-service](serverless.md) compute
platform that you can run anywhere. Some of it's key features:

* Write once
  * [Any language](docs/faq.md#which-languages-are-supported)
  * [AWS Lambda format supported](docs/lambda/README.md)
* [Run anywhere](docs/faq.md#where-can-i-run-functions)
  * Public, private and hybrid cloud
  * [Import functions directly from Lambda](docs/lambda/import.md) and run them wherever you want
* Easy to use [for developers](docs/README.md#for-developers)
* Easy to manage [for operators](docs/README.md#for-operators)
* Written in [Go](https://golang.org)
* Simple yet powerful extensibility


## Prequisites

* Docker 17.05 or later installed and running
* Logged into Docker Hub (`docker login`)

## Usage

### Installation (if running locally)

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

### Installation (if using internal alpha service)

Set your system to point to the internal service on BMC:

```sh
export API_URL=http://129.146.10.253:80
```

Download the pre-built CLI binary:

1. Visit: https://gitlab-odx.oracle.com/odx/functions/tree/master/fn/releases/download/0.3.2
2. Download the CLI for your platform
3. Put in /usr/local/bin
4. chmod +x


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

### Your First Function

Functions are small but powerful blocks of code that generally do one simple thing. Forget about monoliths when using functions, just focus on the task that you want the function to perform.

The following is a simple Go program that outputs a string to STDOUT. Copy and paste the code below into a file called `func.go`. Currently the function must be named func.your_language_extention (ie func.go, func.js, etc.)

```go
package main

import (
	"fmt"
)

func main() {
	fmt.Println("Hello from Oracle Functions!")
}
```

Now run the following CLI commands:

```sh
# Initialize your function
# This detects your runtime from the code above and creates a func.yaml
fn init <DOCKERHUB_USERNAME>/hello

# Test your function
# This will run inside a container exactly how it will on the server
fn run

# Deploy your functions to the Oracle Functions server (default localhost:8080)
# This will create a route to your function as well
fn deploy myapp
```

Now you can call your function:

```sh
curl http://localhost:8080/r/myapp/hello
```

Or in a browser: [http://localhost:8080/r/myapp/hello](http://localhost:8080/r/myapp/hello)

That's it! You just deployed your first function and called it. Now to update your function
you can update your code and run `fn deploy myapp` again.

## To Learn More

- Visit our Functions [Tutorial Series](examples/tutorial/)
- See our [full documentation](docs/README.md)
- View all of our [examples](/examples)
- You can also write your functions in AWS [Lambda format](docs/lambda/README.md)

## Get Involved

- TODO: Slack or Discord community
- Learn how to [contribute](CONTRIBUTING.md)
- See [milestones](https://gitlab-odx.oracle.com/odx/functions/milestones) for detailed issues


## User Interface

This is the graphical user interface for Oracle Functions. It is currently not buildable.

```sh
docker run --rm -it --link functions:api -p 4000:4000 -e "API_URL=http://api:8080" treeder/functions-ui
```

For more information, see: [https://github.com/treeder/functions-ui](https://github.com/treeder/functions-ui)


# Next up

### Check out the [Tutorial Series](examples/tutorial/).

 It will demonstrate some of Oracle Functions capabilities through a series of exmaples. We'll try to show examples in most major languages. This is a great place to start!
