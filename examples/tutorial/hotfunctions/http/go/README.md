# Hot Function Example

This is an example of a hot function. The [hot function documentation](/docs/hot-functions.md) contains an analysis of how this example works.

### How to run the example

Install the CLI tool, start a Fn server and run `docker login` to login to DockerHub. See the [front page](README.md) for instructions. 

Set your Docker Hub username 

```sh
export FN_REGISTRY=<DOCKERHUB_USERNAME>
```

Build and deploy the function to the Fn server (default localhost:8080)

fn deploy --app hot-app
```

Now call your function:

```sh
curl -X POST -d "World" http://localhost:8080/r/hot-app/%2Fhotfn-go
```
