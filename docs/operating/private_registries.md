# Using a private registry with Fn

For local development, or a team that wishes to keep their images off of the public Docker registry, a private
registry may be useful. This can be hosted on your own server or local machine. See the Docker docs [here](https://docs.docker.com/registry/) for information on setting this up. A registry on localhost may greatly speed up iterative development in environments where the network is constrained.

This is where the `FN_REGISTRY` environment variable or `--registry` setting comes into play.

## The `FN_REGISTRY` or `--registry` setting.

This determines where your images will be pushed to or deployed from. It can follow one of the following schemes:

- `myuser` -> `docker.io/myuser/<image>:<tag>`. Used for interacting with the official docker registry.

- `somedomain.com` -> `somedomain.com/<image>:<tag>`. A custom registry hosted at the given domain. The image is not nested under a path.

- `somedomain.com:port` -> `somedomain.com:port/<image>:<tag>`. A custom registry hosted at the given port. (Useful for insecure http registries running on port 80/5000).

- `somedomain.com[port?]/path` -> `somedomain.com[port?]/path/<image>:<tag>`. The image will be nested under the given path. This path can be more than one element.

### Insecure HTTP registries

If your registry is not hosted over https, your Docker daemon must be configured to treat the registry as http only.

In most installations this will require adding the `<hostname>:<port>` to the `insecure_registries` configuration for the Docker daemon. See [here](https://docs.docker.com/registry/insecure/) for more details and troubleshooting. Docker-for-Mac will require changing a setting in the UI.

## Example of a private registry on localhost

Starting a `registry:2` container:

```bash
$ docker run -d -p 5000:5000 --name registry registry:2
$ docker ps
CONTAINER ID        IMAGE               COMMAND                  CREATED             STATUS              PORTS                    NAMES
c18ac6172e0d        registry:2          "/entrypoint.sh /e..."   2 seconds ago       Up 2 seconds        0.0.0.0:5000->5000/tcp   registry
```

Given a function called "dummy":

```bash
$ ls ./dummy
func.go    func.yaml
$ cat ./dummy/func.yaml
version: 0.0.1
runtime: go
entrypoint: ./func
format: http
```

Upload it to your registry (notice the use of `--registry`):

```bash
$ fn deploy --app myapp --registry localhost:5000/some/path
Deploying dummy to app: myapp at path: /dummy
Bumped to version 0.0.2
Building image localhost:5000/some/path/dummy:0.0.2 ..
Pushing localhost:5000/some/path/dummy:0.0.2 to docker registry...The push refers to a repository [localhost:5000/some/path/dummy]
5fcef0dbce8b: Pushed
ce4a1aad8bd7: Pushed
88229188f6e3: Pushed
d82c387bddae: Pushed
0.0.2: digest: sha256:369e158767c89357142f4f394618838b04865af7ab6afc183b38b5f46a0ece3f size: 1155
Updating route /dummy using image localhost:5000/some/path/dummy:0.0.2...
```

Now you can use the route and function named "dummy" as normal.

## Authenticating against private registries

Pushing images to registries that require authentication (like Dockerhub) will require the use of `docker login` from your developer machine. This will be as similar for your private registry if you have authentication enabled.

For pulling images you may also require your Docker daemon to be authenticated. This also needs to be done via `docker login` but the [official documentation](https://docs.docker.com/engine/reference/commandline/login/) demonstrates useful alternatives for automating credential usage via a config file or external credential providers which is suggested for larger fleets of machines.
