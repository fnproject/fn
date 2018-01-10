# Function Examples

The goal of the `fdk`'s are to make it just as easy to write a hot function as
it is a cold one. The best way to showcase this is with an example.

This is an example of a hot function using the fdk-go bindings. The [hot function
documentation](https://github.com/fnproject/fn/blob/master/docs/hot-functions.md)
contains an analysis of how this example works under the hood. With any of the
examples provided here, you may use any format to configure your functions in
`fn` itself. Here we add instructions to set up functions with a 'hot' format.

### How to run the example

Install the CLI tool, start a Fn server and run `docker login` to login to
DockerHub. See the [front page](https://github.com/fnproject/fn) for
instructions.

Initialize the example with an image name you can access:

```sh
fn init --runtime docker --format http --name <DOCKERHUB_USER/image_name>
```

`--format json` will also work here, or if your functions already use json
then adding the fdk will be seamless.

Build and deploy the function to the Fn server (default localhost:8080)

```sh
fn deploy --app hot-app
```

Now call your function (may take a sec to pull image):

```sh
curl -X POST -d '{"name":"Clarice"}' http://localhost:8080/r/hot-app/hello
```

**Note** that this expects you were in a directory named 'hello' (where this
example lives), if this doesn't work, replace 'hello' with your `$PWD` from
the `deploy` command.

Then call it again to see how fast hot functions are!

### Details

If you poke around in the Dockerfile you'll see that we're simply adding the
file found in this directory, getting the `fdk-go` package to our workspace
and then building a binary and building an image with that binary. That then
gets deployed to dockerhub and fn.

For more robust projects, it's recommended to use a tool like `dep` or
`glide` to get dependencies such as the `fdk-go` into your functions.

Scoping out `func.go` you can see that the handler code only deals with input
and output, and doesn't have to deal with decoding the formatting from
functions (i.e. i/o is presented through `io.Writer` and `io.Reader`). This
makes it much easier to write hot functions.

