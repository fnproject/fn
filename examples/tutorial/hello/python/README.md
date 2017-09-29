# Tutorial 1: Python Function w/ Input (3 minutes)

This example will show you how to test and deploy Python code to Fn. It will also demonstrate passing data in through stdin.

### First, run the following commands:

```sh
# Initialize your function creating a func.yaml file
fn init --name hello-python

# Test your function. 
# This will run inside a container exactly how it will on the server. It will also install and vendor dependencies from Gemfile
fn run

# Now try with an input
cat sample.payload.json | fn run

# Deploy your functions to the Fn server (default localhost:8080)
# This will create a route to your function as well
fn deploy --app myapp
```
### Now call your function:

```sh
curl http://localhost:8080/r/myapp/hello-python
```

Or call from a browser: [http://localhost:8080/r/myapp/hello-python](http://localhost:8080/r/myapp/hello-python)

And now with the JSON input:

```sh
curl -H "Content-Type: application/json" -X POST -d @sample.payload.json http://localhost:8080/r/myapp/hello-python
```

That's it! Our `fn deploy` packaged our function and sent it to the Fn server. Try editing `func.py` 
and then doing another `fn deploy`.

### Note on Dependencies

In Python, we create a [requirements](https://pip.pypa.io/en/stable/user_guide/) file in your function directory then `fn deploy` will build and deploy with these dependencies.

# In Review

1. We piped JSON data into the function at the command line
    ```sh
    cat sample.payload.json | fn run
    ```

2. We received our function input through **stdin**
    ```python
    obj = json.loads(sys.stdin.read())
    ```

3. We wrote our output to **stdout**
    ```python
    print "Hello", name, "!"
    ```

4. We sent **stderr** to the server logs
    ```python
    sys.stderr.write("Starting Python Function\n")
    ```


# Next Up
## [Part 2: Input Parameters](../../params)
