# Oracle Functions: Java
This is a hello world example of a Oracle Function using the Java runtime.

Firstly, we initialize our function by creating a `func.yaml` using `fn init`. This command can optionally take a `--runtime` flag to explicitly specify the target function runtime. In this example, the target runtime is implied to be Java because there is a `Func.java` file in the working directory.

```sh
$ fn init <YOUR_DOCKERHUB_USERNAME>/hello-java
```

This is what our `func.yaml` looks like now.


```
name: mhaji/hello-java
version: 0.0.1
runtime: java
entrypoint: java Func
path: /hello-java
max_concurrency: 1
```

Next, we build and run our function using `fn run`.


```sh
$ fn run
Hello, world!
```

You can also pipe input via `stdin` into to the function as follows:

```sh
$ echo "Michael FassBender" | fn run
Hello Michael FassBender!
```

To execute your function via a HTTP trigger:

```sh
fn apps create myapp
fn routes create myapp /hello
curl -H "Content-Type: text/plain" -X POST -d "Michael FassBender" http://localhost:8080/r/myapp/hello
```