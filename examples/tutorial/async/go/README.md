# Aynchronous Function Example

This is an example of an [asynchronous function](/docs/async.md). 

### First, run the following commands:

```sh
# Initialize your function creating a func.yaml file
fn init --type async --name hello-go-async

# Test your function. This will run inside a container exactly how it will on the server
fn run

# Now try with an input
cat sample.payload.json | fn run

# Deploy your functions to the Fn server (default localhost:8080)
# This will create a route to your function as well
fn deploy --app myapp
```

### Now call your function:

```sh
cat payload.json | fn call myapp hello-go-async
```

That's it!
