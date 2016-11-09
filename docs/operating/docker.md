# Docker Configuration

To get the best performance, you'll want to ensure that Docker is configured properly. These are the environments known to produce the best results:

1. Linux 4.7 or newer with aufs or overlay2 module.
2. Ubuntu 16.04 LTS or newer with aufs or overlay2 module.
3. Docker 1.12 or newer to be available.

It is important to reconfigure host's Docker with this filesystem module. Thus, in your Docker start scripts you must do as following:

```
docker daemon [...] --storage-driver=overlay2
```

In case you are using Ubuntu, you can reconfigure Docker easily by updating `/etc/docker/daemon.json` and restarting Docker:

```json
{
    "storage-driver": "overlay2"
}
```
