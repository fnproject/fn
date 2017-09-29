# Tutorial 1: Ruby Function w/ Input (3 minutes)

This example will show you how to test and deploy Ruby code to Fn. It will also demonstrate passing data in through stdin.

### First, run the following commands:

```sh
# Initialize your function creating a func.yaml file
fn init --name hello-ruby

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
curl http://localhost:8080/r/myapp/hello-ruby
```

Or call from a browser: [http://localhost:8080/r/myapp/hello-ruby](http://localhost:8080/r/myapp/hello-ruby)

And now with the JSON input:

```sh
curl -H "Content-Type: application/json" -X POST -d @sample.payload.json http://localhost:8080/r/myapp/hello-ruby
```

That's it! Our `fn deploy` packaged our function and sent it to the Fn server. Try editing `func.rb` 
and then doing another `fn deploy`.


### Note on Dependencies

In Ruby, we create a [Gemfile](http://bundler.io/gemfile.html) file in your function directory. Then any `fn run`
or `fn deploy` will rebuild your gems and vendor them.

Note: Ruby doesn't pick up the gems automatically, so you'll have to add this to the top of your `func.rb` file:

```ruby
require_relative 'bundle/bundler/setup'
```

Open `func.rb` to see it in action.

# In Review

1. We piped JSON data into the function at the command line
    ```sh
    cat sample.payload.json | fn run
    ```

2. We received our function input through **stdin**
    ```ruby
    payload = STDIN.read
    ```

3. We wrote our output to **stdout**
    ```ruby
    puts "Hello #{name} from Ruby!"
    ```

4. We sent **stderr** to the server logs
    ```ruby
    STDERR.puts
    ```

5. We enabled our Ruby gem dependencies using `require_relative`
    ```ruby
    require_relative 'bundle/bundler/setup'
    ```


# Next Up
## [Part 2: Input Parameters](../../params)



