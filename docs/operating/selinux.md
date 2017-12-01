# Running on SELinux systems

Systems such as OEL 7.x where SELinux is enabled and the security policies are set to "Enforcing" will restrict Fn from
running containers and mounting volumes.

For local development, you can relax SELinux constraints by running this command in a root shell:

```sh
setenforce permissive
```

Then you will be able to run `fn start` as normal.

Alternatively, use the docker-in-docker deployment that a production system would use:

```sh
docker run --privileged --rm --name fns -it -v $PWD/data:/app/data -p 8080:8080 fnproject/functions
```

Check the [operating options](options.md) for further details about this.
