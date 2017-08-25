# Fn [![CircleCI](https://circleci.com/gh/fnproject/fn.svg?style=svg&circle-token=6a62ac329bc5b68b484157fbe88df7612ffd9ea0)](https://circleci.com/gh/fnproject/fn)
[![GoDoc](https://godoc.org/github.com/fnproject/fn?status.svg)](https://godoc.org/github.com/fnproject/fn)

Fn is an event-driven, open source, [functions-as-a-service](serverless.md) compute
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

## Quickstart

### Install CLI tool

This isn't required, but it sure makes things a lot easier. Just run the following to install:

```sh
curl -LSs https://raw.githubusercontent.com/fnproject/cli/master/install | sh
```

This will download a shell script and execute it.  If the script asks for a password, that is because it invokes sudo.

### Run Fn Server

Then fire up an Fn server:

```sh
fn start
```

This will start Fn in single server mode, using an embedded database and message queue. You can find all the
configuration options [here](docs/operating/options.md). If you are on Windows, check [here](docs/operating/windows.md).

### Your First Function

Functions are small but powerful blocks of code that generally do one simple thing. Forget about monoliths when using functions, just focus on the task that you want the function to perform.

First, create an empty directory called `hello` and cd into it.

The following is a simple Go program that outputs a string to STDOUT. Copy and paste the code below into a file called `func.go`.

```go
package main

import (
  "fmt"
)

func main() {
  fmt.Println("Hello from Fn!")
}
```

Now run the following CLI commands:

```sh
# Initialize your function
# This detects your runtime from the code above and creates a func.yaml
fn init

# Test your function
# This will run inside a container exactly how it will on the server
fn run

# Set your Docker Hub username
export FN_REGISTRY=<DOCKERHUB_USERNAME>

# Deploy your functions to the Fn server (default localhost:8080)
# This will create a route to your function as well
fn deploy myapp
```

Now you can call your function:

```sh
curl http://localhost:8080/r/myapp/hello
# or:
fn call myapp /hello
```

Or in a browser: [http://localhost:8080/r/myapp/hello](http://localhost:8080/r/myapp/hello)

That's it! You just deployed your first function and called it. To update your function
you can update your code and run `fn deploy myapp` again.

## To Learn More

* Visit our Functions [Tutorial Series](examples/tutorial/)
* See our [full documentation](docs/README.md)
* View all of our [examples](/examples)
* You can also write your functions in AWS [Lambda format](docs/lambda/README.md)

## Get Involved

- Join our [Slack Community](https://join.slack.com/t/fnproject/shared_invite/MjIwNzc5MTE4ODg3LTE1MDE0NTUyNTktYThmYmRjZDUwOQ)
- Learn how to [contribute](CONTRIBUTING.md)
- See [milestones](https://github.com/fnproject/fn/milestones) for detailed issues

## User Interface

This is the graphical user interface for Fn. It is currently not buildable.

```sh
docker run --rm -it --link functions:api -p 4000:4000 -e "API_URL=http://api:8080" treeder/functions-ui
```

For more information, see: [https://github.com/treeder/functions-ui](https://github.com/treeder/functions-ui)

## Next up

### Check out the [Tutorial Series](examples/tutorial/)

 It will demonstrate some of Fn capabilities through a series of exmaples. We'll try to show examples in most major languages. This is a great place to start!

