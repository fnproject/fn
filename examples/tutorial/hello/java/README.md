# Fn: Java
This example will show you how to test and deploy Java code to Fn. It will also demonstrate passing data in through stdin.

### First, run the following commands:

```sh
# Initialize your function creating a func.yaml file
fn init --name hello-java --runtime java

# Test your function. This will run inside a container exactly how it will on the server
fn run

# Now try with an input
echo "Michael FassBender" | fn run

# Deploy your functions to the Fn server (default localhost:8080)
# This will create a route to your function as well
fn deploy --app myapp
```

### Now call your function:

```sh
curl http://localhost:8080/r/myapp/hello-java
```

Or call from a browser: [http://localhost:8080/r/myapp/hello-java](http://localhost:8080/r/myapp/hello-java)

That's it!


# In Review

1. We passed in data through stdin
    ```sh
    echo "Michael FassBender" | fn run
    ```

2. We received our function input through **stdin**
    ```go
    String name = bufferedReader.readLine();
    ```

3. We wrote our output to **stdout**
    ```go
    System.out.println("Hello, " + name + "!");
    ```


# Next Up
## [Part 2: Input Parameters](../../params)
