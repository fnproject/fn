# Go Function Hello World

This example will show you how to test and deploy Go code to Fn. It will also demonstrate passing JSON data in through `stdin`.

This tutorial assumes you have installed Docker, Fn server, and Fn CLI. See the [Fn Quickstart](https://github.com/fnproject/fn) for installation steps.

## Start Fn Server

Start up the Fn server so we can deploy our function.

```sh
fn start
```

The command starts Fn in single server mode using an embedded database and message queue. You can find all the
configuration options [here](docs/operating/options.md). If you are on Windows, check [here](docs/operating/windows.md).

## Create your Function 

1. Create an empty directory called `hello` and cd into it.
1. Create a file for the Go function: `func.go`.
1. Add the following code to the file.

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
	mapD := map[string]string{"message": fmt.Sprintf("Hello %s!", p.Name)}
	mapB, _ := json.Marshal(mapD)
	fmt.Println(string(mapB))
}
```
<ol start="4">
  <li>Initialize your function file by entering.</li>
</ol>

    fn init

Which returns

```sh
Found go, assuming go runtime.
func.yaml created
```

Fn found your `func.go` file and generated a `func.yaml` file with contents that should look like:

```yaml
name: hello
version: 0.0.1
runtime: go
entrypoint: ./func
Understanding func.yaml
```

The generated `func.yaml` file contains metadata about your function and declares a number of properties including:

* the name of your function: taken from the containing directory name
* the version: automatically starting at 0.0.1
* the name of the runtime/language: which was set automatically based on the presence of func.go
* the name of the function to invoke: in this case ./func which will be the name of the compiled Go file.

There are other user specifiable properties but these will suffice for this example.

## Test your Function

Test your function using the following command.

    fn run

Fn runs your function inside a container exactly how it executes on the server.

Run the function again with JSON input.

    echo '{"name":"joe"}' | fn run

## Deploy your Function to Fn Server

Deploy your functions to the Fn server. 

    fn deploy --app myapp

The command creates an app on the server named `myapp`. In addition, a route to your function created based on your directory name `/hello`.

## Test your Function on the Server

With the function deployed to the server, you can make calls to the function. 

### Call your Function without Input

Call your function using the Fn CLI.

    fn call myapp /hello

Open a web browser and enter <http://localhost:8080/r/myapp/hello>.

Or try `curl`.
    
    curl http://localhost:8080/r/myapp/hello

All of these options should return:

    {"message":"Hello World!"}
    
### Call your Function with Input

You can use `curl` to pass JSON data to your function.

```sh
curl -X POST -d '{"name":"Johnny"}' -H "Content-Type: application/json" http://localhost:8080/r/myapp/hello
```
Or specify a file.

```sh
curl -X POST -d @name.json -H "Content-Type: application/json" http://localhost:8080/r/myapp/hello
```
Both commands should return:

```json
{"message":"Hello Johnny!"}
```

## Function Complete

That's it! You have coded your first Go function.

