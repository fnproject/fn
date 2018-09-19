![Fn Project](http://fnproject.io/images/fn-300x125.png)

**[Quickstart](https://github.com/fnproject/fn#quickstart)&nbsp; | &nbsp;[Tutorials](https://fnproject.io/tutorials)&nbsp; |  &nbsp;[Docs](https://github.com/fnproject/fn/blob/master/docs/README.md)&nbsp; | &nbsp;[API](http://petstore.swagger.io/?url=https://raw.githubusercontent.com/fnproject/fn/master/docs/swagger.yml)&nbsp; | &nbsp;[Operating](https://github.com/fnproject/fn/blob/master/docs/README.md#for-operators)&nbsp; | &nbsp;[Flow](https://github.com/fnproject/flow)&nbsp; | &nbsp;[UI](https://github.com/fnproject/ui)**

[![CircleCI](https://circleci.com/gh/fnproject/fn.svg?style=svg&circle-token=6a62ac329bc5b68b484157fbe88df7612ffd9ea0)](https://circleci.com/gh/fnproject/fn) [![GoDoc](https://godoc.org/github.com/fnproject/fn?status.svg)](https://godoc.org/github.com/fnproject/fn)
[![Go Report Card](https://goreportcard.com/badge/github.com/fnproject/fn)](https://goreportcard.com/report/github.com/fnproject/fn)

## Welcome
Fn is an event-driven, open source, [Functions-as-a-Service (FaaS)](docs/serverless.md) compute platform that you can run anywhere. Some of its key features:

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

The fastest way to experience Fn is to follow the quickstart below, or you can jump right to our [full documentation](docs/README.md), [API Docs](http://petstore.swagger.io/?url=https://raw.githubusercontent.com/fnproject/fn/master/docs/swagger.yml), or hit us up in our [Slack Community](http://slack.fnproject.io)!


## Quickstart

### Pre-requisites

* Docker 17.10.0-ce or later installed and running
* A Docker Hub account ([Docker Hub](https://hub.docker.com/)) (or other Docker-compliant registry)
* Log Docker into your Docker Hub account: `docker login`

### Install CLI tool

The command line tool isn't required, but it sure makes things a lot easier. There are a few options to install it:

#### 1. Homebrew - macOS

If you're on a Mac and use [Homebrew](https://brew.sh/), this one is for you:

```sh
brew install fn
```

#### 2. Shell script - Linux and macOS

This one works on Linux and macOS (partially on Windows).

If you are running behind a proxy first set your http_proxy and https_proxy environmental variables:

```sh
curl -LSs https://raw.githubusercontent.com/fnproject/cli/master/install | sh
```

This will download a shell script and execute it. If the script asks for a password, that is because it invokes sudo.


#### 3. Download the bin - Linux, macOS and Windows

Head over to our [releases](https://github.com/fnproject/cli/releases) and download it.

### Run Fn Server

Now fire up an Fn server:

```sh
fn start
```

This will start Fn in single server mode, using an embedded database and message queue. You can find all the
configuration options [here](docs/operating/options.md). If you are on Windows, check [here](docs/operating/windows.md).
If you are on a Linux system where the SELinux security policy is set to "Enforcing", such as Oracle Linux 7, check
[here](docs/operating/selinux.md).

### Your First Function

Functions are small but powerful blocks of code that generally do one simple thing. Forget about monoliths when using functions, just focus on the task that you want the function to perform. Our CLI tool will help you get started super quickly.

Create hello world function:

```sh
fn init --runtime go hello
```

This will create a simple function in the directory `hello`, so let's cd into it:

```sh
cd hello
```

Feel free to check out the files it created or just keep going and look at it later.

```sh
# Set your Docker Hub username
export FN_REGISTRY=<DOCKERHUB_USERNAME>

# Run your function locally
fn run

# Deploy your functions to your local Fn server
fn deploy --app myapp --local
```

Now you can call your function:

```sh
fn invoke myapp hello
```

That's it! You just deployed your first function and called it. Try updating the function code in `func.go` then deploy it again to see the change.

## Learn More

* With our [Fn Getting Started Series](examples/tutorial/), quickly create Fn Hello World applications in multiple languages. This is a great Fn place to start!
* Visit [Fn tutorials](http://fnproject.io/tutorials) for step by step guides to creating apps with Fn . These tutorials range from introductory to more advanced.
* See our [full documentation](docs/README.md)
* View all of our [examples](/examples)
* View our [YouTube Channel](https://www.youtube.com/channel/UCo3fJqEGRx9PW_ODXk3b1nw)
* View our [API Docs](http://petstore.swagger.io/?url=https://raw.githubusercontent.com/fnproject/fn/master/docs/swagger.yml)
* Check out our sub-projects: [Flow](https://github.com/fnproject/flow), [UI](https://github.com/fnproject/ui), [FnLB](https://github.com/fnproject/lb)
* For a full presentation with lots of content you can use in your own presentations, see [The Fn Project Presentation Master](http://deck.fnproject.io)


## Get Help

* [Ask your question on StackOverflow](https://stackoverflow.com/questions/tagged/fn) and tag it with `fn`
* Join our [Slack Community](https://join.slack.com/t/fnproject/shared_invite/MjIwNzc5MTE4ODg3LTE1MDE0NTUyNTktYThmYmRjZDUwOQ)

## Get Involved

* Join our [Slack Community](http://slack.fnproject.io)
* Learn how to [contribute](CONTRIBUTING.md)
* See [issues](https://github.com/fnproject/fn/issues) for issues you can help with
* Join us at one of our [Fn Events](http://events.fnproject.io) or even speak at one!

## Stay Informed

* [Blog](https://medium.com/fnproject)
* [Twitter](https://twitter.com/fnproj)
* [YouTube](https://www.youtube.com/channel/UCo3fJqEGRx9PW_ODXk3b1nw)
* [Events](http://events.fnproject.io)
