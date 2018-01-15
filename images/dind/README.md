# Docker-in-Docker Base Image

This is the base image for all docker-in-docker images.

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
