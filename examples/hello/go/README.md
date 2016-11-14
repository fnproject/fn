# Quick Example for a Go Function (3 minutes)

This example will show you how to test and deploy Go (Golang) code to IronFunctions.

```sh
fnctl init <YOUR_DOCKERHUB_USERNAME>/hello
fnctl build
# test it
cat hello.payload.json | fnctl run
# push it to Docker Hub
fnctl push
# Create a route to this function on IronFunctions
fnctl routes create myapp /hello YOUR_DOCKERHUB_USERNAME/hello:0.0.X
# todo: Image name could be optional if we read the function file for creating the route. Then command could be:
fnctl routes create myapp /hello
```

Now you use your function on IronFunctions:

 ```sh
 curl -H "Content-Type: application/json" -X POST -d @hello.payload.json http://localhost:8080/r/myapp/hello
 ```

Or surf to it: http://localhost:8080/r/myapp/hello
