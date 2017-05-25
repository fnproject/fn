# Oracle Functions

[![GoDoc](https://godoc.org/github.com/treeder/functions?status.svg)](https://godoc.org/github.com/treeder/functions)

Welcome to Oracle Functions! The open source serverless platform.

## What is Oracle Functions?

Oracle Functions is an open source [serverless](serverless.md) platform, or as we like to refer to it, Functions as a
Service (FaaS) platform that you can run anywhere.

* Write once
  * [Any language](docs/faq.md#which-languages-are-supported)
  * [AWS Lambda format supported](docs/lambda/README.md)
* [Run anywhere](docs/faq.md#where-can-i-run-functions)
  * Public, private and hybrid cloud
  * [Import functions directly from Lambda](docs/lambda/import.md) and run them wherever you want
* Easy to use [for developers](docs/README.md#for-developers)
* Easy to manage [for operators](docs/README.md#for-operators)
* Written in [Go](https://golang.org)

## Join Our Community

TODO: Slack or Discord community. 

## Prequisites

* Docker 17.05 or later installed and running
* Logged into Docker Hub (`docker login`)

## Installation 

The following instructions apply while the project is a private repo. They'll
build Oracle Functions and the CLI tool directly from the repo instead of
using pre-built containers. Will be much easier once public.

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

### Write a Function

Functions are small, bite sized bits of code that do one simple thing. Forget about monoliths when using functions,
just focus on the task that you want the function to perform.

The following is a Go function that just returns "Hello ${NAME}!":

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
	fmt.Printf("Hello %v!", p.Name)
}
```

Copy and paste the code above into a file called `func.go`, then run the following commands to build your function
and deploy it.

```sh
# Initilize your function, replace $USERNAME with your Docker Hub username.
fn init $USERNAME/hello
# Test it - you can pass data into it too by piping it in, eg: `cat hello.payload.json | fn run`
fn run
# Once it's ready, deploy it to your functions server (default localhost:8080)
fn apps create myapp
fn deploy myapp
```

Now you can call your function:

```sh
curl http://localhost:8080/r/myapp/hello
```

Or surf to it: http://localhost:8080/r/myapp/hello

To update your function:

```sh
# Just update your code and run:
fn deploy myapp
```

See the [documentation](docs/README.md) for more information. And you can find a bunch of examples in various languages in the [examples](examples/) directory. You can also
write your functions in AWS's [Lambda format](docs/lambda/README.md).

## Functions UI

```sh
docker run --rm -it --link functions:api -p 4000:4000 -e "API_URL=http://api:8080" treeder/functions-ui
```

For more information, see: https://github.com/treeder/functions-ui

## Writing Functions

See [Writing Functions](docs/writing.md).

And you can find a bunch of examples in the [/examples](/examples) directory.

## More Documentation

See [docs/](docs/README.md) for full documentation.

## Roadmap

See [milestones](https://gitlab.oracledx.com/odx/functions/milestones) for detailed issues.


## Want to contribute to Oracle Functions?

See [contributing](CONTRIBUTING.md).
