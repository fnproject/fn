# Tutorial 1: NodeJS Function w/ Input (3 minutes)

This example will show you how to test and deploy Node code to Fn. It will also demonstrate passing data in through stdin.

### First, run the following commands:

```sh
# Initialize your function creating a func.yaml file
fn init --name hello-node

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
curl http://localhost:8080/r/myapp/hello-node
```

Or call from a browser: [http://localhost:8080/r/myapp/hello-node](http://localhost:8080/r/myapp/hello-node)

And now with the JSON input:

```sh
curl -H "Content-Type: application/json" -X POST -d @sample.payload.json http://localhost:8080/r/myapp/hello-node
```

That's it! Our `fn deploy` packaged our function and sent it to the Fn server. Try editing `func.js` 
and then doing another `fn deploy`.

### Note on Dependencies

Create a [package.json](https://docs.npmjs.com/getting-started/using-a-package.json) file in your functions directory. The CLI should pick that up and build in all
your dependencies on `fn run` and `fn deploy`.

For example, using the `package.json` file in this directory which includes the [request](https://www.npmjs.com/package/request) package, you can add this to func.js and it will work:

```js
var request = require('request');
request('http://www.google.com', function (error, response, body) {
  if (!error && response.statusCode == 200) {
    console.log(body) // Show the HTML for the Google homepage.
  }
})
```


# In Review

1. We piped JSON data into the function at the command line
    ```sh
    cat sample.payload.json | fn run
    ```

2. We received our function input through **stdin**
    ```node
    obj = JSON.parse(fs.readFileSync('/dev/stdin').toString())
    ```

3. We wrote our output to **stdout**
    ```node
    console.log
    ```

4. We sent **stderr** to the server logs
    ```node
    console.error
    ```


# Next Up
## [Part 2: Input Parameters](../../params)



