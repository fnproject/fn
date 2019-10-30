<a id="top"></a>
![Fn Project](http://fnproject.io/images/fn-300x125.png)

**[Quickstart](https://github.com/fnproject/fn#quickstart)&nbsp; | &nbsp;[Tutorials](https://fnproject.io/tutorials)&nbsp; |  &nbsp;[Docs](https://github.com/fnproject/docs)&nbsp; | &nbsp;[API](http://petstore.swagger.io/?url=https://raw.githubusercontent.com/fnproject/fn/master/docs/swagger_v2.yml)&nbsp; | &nbsp;[Operating](https://github.com/fnproject/docs/blob/master/fn/README.md#for-operators)&nbsp; | &nbsp;[Flow](https://github.com/fnproject/flow)&nbsp; | &nbsp;[UI](https://github.com/fnproject/ui)**

[![CircleCI](https://circleci.com/gh/fnproject/fn.svg?style=svg&circle-token=6a62ac329bc5b68b484157fbe88df7612ffd9ea0)](https://circleci.com/gh/fnproject/fn) [![GoDoc](https://godoc.org/github.com/fnproject/fn?status.svg)](https://godoc.org/github.com/fnproject/fn)
[![Go Report Card](https://goreportcard.com/badge/github.com/fnproject/fn)](https://goreportcard.com/report/github.com/fnproject/fn)

## Welcome
Fn is an event-driven, open source, [Functions-as-a-Service (FaaS)](https://github.com/fnproject/docs/blob/master/fn/general/introduction.md) compute platform that you can run anywhere. Some of its key features:

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

The fastest way to experience Fn is to follow the quickstart below, or you can jump right to our [full documentation](https://github.com/fnproject/docs), [API Docs](http://petstore.swagger.io/?url=https://raw.githubusercontent.com/fnproject/fn/master/docs/swagger_v2.yml), or hit us up in our [Slack Community](http://slack.fnproject.io) or [Community Page](https://github.com/fnproject/docs/blob/master/COMMUNITY.md)!


## Quickstart

### Pre-requisites

* Docker 17.10.0-ce or later installed and running
* [Docker Hub](https://hub.docker.com/) account (or other Docker-compliant registry) (Not required for local development)
* Logged into Registry: ie `docker login` (Not required for local development)

### Install CLI tool

The command line tool isn't required, but it makes things a lot easier. There are a few options to install it:

#### Option 1. Homebrew - macOS

If you're on a Mac and use [Homebrew](https://brew.sh/):

```sh
brew update && brew install fn
```

#### Option 2. Shell script - Linux and macOS

This one works on Linux and macOS (partially on Windows).

If you are running behind a proxy first set your http_proxy and https_proxy environment vars:

```sh
curl -LSs https://raw.githubusercontent.com/fnproject/cli/master/install | sh
```

This will download a shell script and execute it. If the script asks for a password, that is because it invokes sudo.

#### Option 3. Install the Windows CLI
[Install and run the Fn Client for Windows](https://github.com/fnproject/docs/blob/master/fn/develop/running-fn-client-windows.md).


#### Option 4. Download the bin - Linux, macOS and Windows
Head over to our [releases](https://github.com/fnproject/cli/releases) and download it.


### Run Fn Server

First, start up an Fn server locally:

```sh
fn start
```

This will start Fn in single server mode, using an embedded database and message queue. You can find all the
configuration options [here](https://github.com/fnproject/docs/blob/master/fn/operate/options.md). If you are on Windows, check [here](https://github.com/fnproject/docs/blob/master/fn/operate/windows.md).
If you are on a Linux system where the SELinux security policy is set to "Enforcing", such as Oracle Linux 7, check
[here](https://github.com/fnproject/docs/blob/master/fn/operate/selinux.md).

### Your First Function

Functions are small but powerful blocks of code that generally do one simple thing. Forget about monoliths when using functions, just focus on the task that you want the function to perform. Our CLI tool will help you get started quickly.

Let's create your function. You can use any runtime (ie go, node, java, python, etc.) `hello` will be the name of your function as well as create a directory called `hello`. You can name your function anything.

```sh
fn init --runtime go hello
cd hello
```

We need to create an "app" which acts as a top-level collection of functions and other elements:

```sh
fn create app myapp
```

Deploy your function: 

```sh
fn deploy --app myapp --local
```

Note: `--local` flag will skip the push to remote container registry making local development faster

Now let's actually run your function using the `invoke` command:

```sh
fn invoke myapp hello
```

That's it! You just deployed and ran your first function! Try updating the function code in `func.go` (or .js, .java, etc.) then deploy it again to see the change.

## Learn More

* Visit [Fn tutorials](http://fnproject.io/tutorials) for step-by-step guides to creating apps with Fn. These tutorials range from introductory to more advanced.
* See our [full documentation](https://github.com/fnproject/docs)
* View our [YouTube Channel](https://www.youtube.com/channel/UCo3fJqEGRx9PW_ODXk3b1nw)
* View our [API Docs](http://petstore.swagger.io/?url=https://raw.githubusercontent.com/fnproject/fn/master/docs/swagger_v2.yml)
* Check out our sub-projects: [Flow](https://github.com/fnproject/flow), [UI](https://github.com/fnproject/ui), [FnLB](https://github.com/fnproject/lb)
* For a full presentation with lots of content you can use in your own presentations, see [The Fn Project Presentation Master](http://deck.fnproject.io)


## Get Help

* [Ask your question on StackOverflow](https://stackoverflow.com/questions/tagged/fn) and tag it with `fn`

## Get Involved

* Join our [Slack Community](http://slack.fnproject.io)
* See our new [Community Page](https://github.com/fnproject/docs/blob/master/community/)
* Learn how to [contribute](https://github.com/fnproject/docs/blob/master/community/CONTRIBUTING.md)
* Find [issues](https://github.com/fnproject/fn/issues) and become a contributor
* Join us at one of our [Fn Events](http://events.fnproject.io) or even speak at one!
* Coming in Q1'19: Regularly scheduled planning meetings for contributing to the Fn Project

## Stay Informed

* [Blog](https://medium.com/fnproject)
* [Twitter](https://twitter.com/fnproject)
* [YouTube](https://www.youtube.com/channel/UCo3fJqEGRx9PW_ODXk3b1nw)
* [Events](http://events.fnproject.io)
