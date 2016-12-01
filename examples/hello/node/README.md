## Quick Example for a NodeJS Function (4 minutes)

This example will show you how to test and deploy a Node function to IronFunctions.

```sh
# create your func.yaml file
fn init <YOUR_DOCKERHUB_USERNAME>/hello
# build the function
fn build
# test it
cat hello.payload.json | fn run
# push it to Docker Hub
fn push
# Create a route to this function on IronFunctions
fn routes create myapp /hello
```

Now surf to: http://localhost:8080/r/myapp/hello

## Dependencies

Create a [package.json](https://docs.npmjs.com/getting-started/using-a-package.json) file in your functions directory.

Run:

```sh
docker run --rm -v "$PWD":/function -w /function iron/node:dev npm install
```

Then everything should work.

For example, using the `package.json` file in this directory which includes the [request](https://www.npmjs.com/package/request) package, you can add this to func.js and it will work:

```js
var request = require('request');
request('http://www.google.com', function (error, response, body) {
  if (!error && response.statusCode == 200) {
    console.log(body) // Show the HTML for the Google homepage.
  }
})
```
