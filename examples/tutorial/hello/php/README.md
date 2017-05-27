# Tutorial 1: PHP Function w/ Input (3 minutes)

This example will show you how to test and deploy PHP code to Oracle Functions. It will also demonstrate passing data in through stdin.

### First, run the following commands:

```sh
# Initialize your function creating a func.yaml file
fn init <DOCKERHUB_USERNAME>/hello

# Test your function. 
# This will run inside a container exactly how it will on the server. It will also install and vendor dependencies from Gemfile
fn run

# Now try with an input
cat hello.payload.json | fn run

# Deploy your functions to the Oracle Functions server (default localhost:8080)
# This will create a route to your function as well
fn deploy myapp
```
### Now call your function:

```sh
curl http://localhost:8080/r/myapp/hello
```

Or call from a browser: [http://localhost:8080/r/myapp/hello](http://localhost:8080/r/myapp/hello)

And now with the JSON input:

```sh
curl -H "Content-Type: application/json" -X POST -d @hello.payload.json http://localhost:8080/r/myapp/hello
```

That's it!

### Note on Dependencies

In PHP, you can create a [composer](https://getcomposer.org/) file in your function directory, then run:

```sh
fn build
```

This will rebuild your gems and vendor them. PHP doesn't pick them up automatically, so you'll have to add this to the top of your `func.php` file:

```php
require 'vendor/autoload.php';
```

Open `func.php` to see it in action.


### 3. Queue jobs for your function

Now you can start jobs on your function. Let's quickly queue up a job to try it out.

```sh
cat hello.payload.json | fn call phpapp /hello
```



# In Review

1. We piped JSON data into the function at the command line
    ```sh
    cat hello.payload.json | fn run
    ```

2. We received our function input through **stdin**
    ```php
    $payload = json_decode(file_get_contents("php://stdin"), true);
    ```

3. We wrote our output to **stdout**
    ```php
    echo "Hello World!\n";
    ```

4. We sent **stderr** to the server logs
    ```php
    fwrite(STDERR, "--> this will go to stderr (server logs)\n");
    ```

5. We added PHP dependencies and enabled them using:
    ```php
    require 'vendor/autoload.php';
    ```


# Next Up
## [Tutorial 2: Input Parameters](examples/tutorial/params)
