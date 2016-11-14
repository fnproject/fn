## Quick Example for a NodeJS Function (4 minutes)

This example will show you how to test and deploy a Node function to IronFunctions.

```sh
fnctl init <YOUR_DOCKERHUB_USERNAME>/hello
fnctl build
# test it
cat hello.payload.json | fnctl run
# push it to Docker Hub for use with IronFunctions
fnctl push
# Create a route to this function on IronFunctions
fnctl routes create myapp /hello YOUR_DOCKERHUB_USERNAME/hello:0.0.X
# todo: Image name could be optional if we read the function file for creating the route. Then command could be:
fnctl routes create myapp /hello
```

Now surf to: http://localhost:8080/r/myapp/hello
