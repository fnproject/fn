![Fn Project](http://fnproject.io/images/fn-300x125.png)

[![CircleCI](https://circleci.com/gh/fnproject/fn.svg?style=svg&circle-token=6a62ac329bc5b68b484157fbe88df7612ffd9ea0)](https://circleci.com/gh/fnproject/fn) [![GoDoc](https://godoc.org/github.com/fnproject/fn?status.svg)](https://godoc.org/github.com/fnproject/fn)
[![Go Report Card](https://goreportcard.com/badge/github.com/fnproject/fn)](https://goreportcard.com/report/github.com/fnproject/fn)

Fn is an event-driven, open source, [functions-as-a-service](docs/serverless.md) compute
platform that you can run anywhere. Some of it's key features:

* Open Source
* Native Docker: use any Docker container as your Function
* Supports all languages
* Run anywhere
  * Public, private and hybrid cloud
  * Import Lambda functions and run them anywhere
* Easy to use for developers
* Easy to manage for operators
* Written in Go
* Simple yet powerful extensibility

The fastest way to experience Fn is to follow the quickstart below, or you can jump right to our [full documentation](docs/README.md), [API Docs](http://petstore.swagger.io/?url=https://raw.githubusercontent.com/fnproject/fn/master/docs/swagger.yml), or his us up in our [Slack Community](http://slack.fnproject.io)!


## Quickstart

### Pre-requisites

* Docker 17.05 or later installed and running
* A Docker Hub account ([Docker Hub](https://hub.docker.com/)) (or other Docker-compliant registry)
* Log Docker into your Docker Hub account: `docker login`

### Install CLI tool

The command line tool isn't required, but it sure makes things a lot easier. There are a few options to install it:

#### 1. Homebrew - MacOS

If you're on a Mac and use [Homebrew](https://brew.sh/), this one is for you:

```sh
brew install fn
```

#### 2. Shell script

This one works on Linux and MacOS (partially on Windows):

```sh
curl -LSs https://raw.githubusercontent.com/fnproject/cli/master/install | sh
```

This will download a shell script and execute it. If the script asks for a password, that is because it invokes sudo.

#### 3. Download the bin

Head over to our [releases](https://github.com/fnproject/cli/releases) and download it.

### Run Fn Server

Now fire up an Fn server:

```sh
fn start
```

This will start Fn in single server mode, using an embedded database and message queue. You can find all the
configuration options [here](docs/operating/options.md). If you are on Windows, check [here](docs/operating/windows.md).
If you are on a Linux system where the SELinux security policy is set to "Enforcing", such as OEL7.x, check
[here](docs/operating/selinux.md).

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

# Set your Docker Hub username
export FN_REGISTRY=<DOCKERHUB_USERNAME>

# Test your function
# This will run inside a container exactly how it will on the server
fn run

# Deploy your functions to the Fn server (default localhost:8080)
# This will create a route to your function as well
fn deploy --app myapp
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

## User Interface

We also have an open source graphical user interface for Fn. It's very easy to use, simply run the command below:

```sh
docker run --rm -it --link functions:api -p 4000:4000 -e "FN_API_URL=http://api:8080" fnproject/ui
```

For more information, see: [https://github.com/fnproject/ui](https://github.com/fnproject/ui)


## Learn More

* With our [Fn Getting Started Series](examples/tutorial/), quickly create Fn Hello World applications in multiple languages. This is a great Fn place to start!
* Visit [Fn tutorials](https://github.com/fnproject/tutorials) for step by step guides to creating apps with Fn . These tutorials range from introductory to more advanced. 
* See our [full documentation](docs/README.md)
* View all of our [examples](/examples)
* View our [YouTube Channel](https://www.youtube.com/channel/UCo3fJqEGRx9PW_ODXk3b1nw)
* View our [API Docs](http://petstore.swagger.io/?url=https://raw.githubusercontent.com/fnproject/fn/master/docs/swagger.yml)

## Get Help

* [Ask your question on StackOverflow](https://stackoverflow.com/questions/tagged/fn) and tag it with `fn`
* Join our [Slack Community](https://join.slack.com/t/fnproject/shared_invite/MjIwNzc5MTE4ODg3LTE1MDE0NTUyNTktYThmYmRjZDUwOQ)

## Get Involved

* Join our [Slack Community](http://slack.fnproject.io)
* Learn how to [contribute](CONTRIBUTING.md)
* See [milestones](https://github.com/fnproject/fn/milestones) for detailed issues

## Stay Informed

* [Blog](https://medium.com/fnproject)
* [Twitter](https://twitter.com/fnproj)
* [YouTube](https://www.youtube.com/channel/UCo3fJqEGRx9PW_ODXk3b1nw)
