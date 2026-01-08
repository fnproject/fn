# Docker-in-Docker Base Image for local development

This is the base image for all docker-in-docker images. The Dockerfile is used for local development and not used in
build pipeline. The local image is required if you need to build other local image such as images/runner/Dockerfile.

FnServer image has dependency on the preentry.sh script and include that script in multi-arch build.

The difference between this and the official `docker` images are that this will choose the best
filesystem automatically. The official ones use `vfs` (bad) by default unless you pass in a flag.

It will also attempt to mirror the default external interface's MTU to the dind network; this
addresses a problem with running dind-based images on a kubernetes cluster with an overlay
network that takes a chunk out of pods' MTUs.

## Usage

Just use this as your base image and use CMD for your program, **NOT ENTRYPOINT**. This will handle the rest.

```Dockerfile
FROM fnproject/dind
# OTHER STUFF
CMD ["./myproggie"]
```
