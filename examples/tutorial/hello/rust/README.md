# Tutorial 1: Rust Function w/ Input (3 minutes)

This example will show you how to test and deploy Rust code to Fn. It will also demonstrate passing data in through stdin.

The easiest way to create a function in rust is via ***cargo*** and ***fn***.

### Prerequisites

Create an empty rust project as follows:

```bash
cargo init --name func --bin
```

Make sure the project name is ***func*** and is of type ***bin***.

Now put the following code in ```main.rs``` (can also copy directly from the file in this repo):

```rust
use std::io;
use std::io::Read;

fn main() {
    let mut buffer = String::new();
    let stdin = io::stdin();
    if stdin.lock().read_to_string(&mut buffer).is_ok() {
        println!("Hello {}", buffer.trim());
    }
}
```


### Now run the following commands:

```sh
# Initialize your function creating a func.yaml file
fn init --name hello-rust

# Test your function. This will run inside a container exactly how it will on the server
fn run

# Now try with an input (copy sample.payload.json from this repo)
cat sample.payload.json | fn run

# Deploy your functions to the Fn server (default localhost:8080)
# This will create a route to your function as well
fn deploy --app myapp
```
### Now call your function:

```sh
curl http://localhost:8080/r/myapp/hello-rust
```

Or call from a browser: [http://localhost:8080/r/myapp/hello-rust](http://localhost:8080/r/myapp/hello-rust)

And now with the JSON input:

```sh
curl -H "Content-Type: application/json" -X POST -d @sample.payload.json http://localhost:8080/r/myapp/hello-rust
```

That's it!

### Note on Dependencies



# In Review

1. We piped JSON data into the function at the command line
    ```sh
    cat sample.payload.json | fn run
    ```

2. We received our function input through **stdin**
    ```rust
    read_to_string(&mut buffer)
    ```

3. We wrote our output to **stdout**
    ```rust
    println!("Hello {}", buffer.trim());
    ```

4. We sent **stderr** to the server logs
    ```rust
    TODO
    ```


# Next Up
## [Part 2: Input Parameters](../../params)
