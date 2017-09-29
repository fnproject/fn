# Tutorial 1: Go Function w/ Input (3 minutes)

This example will show you how to test and deploy Go (Golang) code to Fn. It will also demonstrate passing data in through stdin.

### First, run the following commands:

```sh
# Initialize your function creating a func.yaml file
fn init --name hello-go

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
curl http://localhost:8080/r/myapp/hello-go
```

Or call from a browser: [http://localhost:8080/r/myapp/go](http://localhost:8080/r/myapp/hello-go)

And now with the JSON input:

```sh
curl -H "Content-Type: application/json" -X POST -d @sample.payload.json http://localhost:8080/r/myapp/hello-go
```

That's it!

### Note on Dependencies

In Go, simply put them all in the `vendor/` directory.

# In Review

1. We piped JSON data into the function at the command line
    ```sh
    cat sample.payload.json | fn run
    ```

2. We received our function input through **stdin**
    ```go
    json.NewDecoder(os.Stdin).Decode(p)
    ```

3. We wrote our output to **stdout**
    ```go
    fmt.Printf("Hello")
    ```

4. We sent **stderr** to the server logs
    ```go
    log.Println("here")
    ```


# Next Up
## [Part 2: Input Parameters](../../params)
