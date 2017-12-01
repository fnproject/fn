# Fn Runtime Options

## Default run command for production

This will run with docker in docker.

```sh
docker run --privileged --rm --name fns -it -v $PWD/data:/app/data -p 80:8080 fnproject/fnserver
```

See below for starting without docker in docker.

## Configuration

When starting Fn, you can pass in the following configuration variables as environment variables. Use `-e VAR_NAME=VALUE` in
docker run.  For example:

```sh
docker run -e VAR_NAME=VALUE ...
```

| Env Variables | Description | Default values |
| --------------|-------------|----------------|
| `FN_DB_URL` | The database URL to use in URL format. See [Databases](databases/README.md) for more information. | sqlite3:///app/data/fn.db |
| `FN_MQ_URL` | The message queue to use in URL format. See [Message Queues](mqs/README.md) for more information. | bolt:///app/data/worker_mq.db |
| `FN_API_URL` | The primary Fn API URL to that this instance will talk to. In a production environment, this would be your load balancer URL. | N/A |
| `FN_PORT `| Sets the port to run on | 8080 |
| `FN_LOG_LEVEL` | Set to DEBUG to enable debugging | INFO |
| `DOCKER_HOST` | Docker remote API URL | /var/run/docker.sock |
| `DOCKER_API_VERSION` | Docker remote API version | 1.24 |
| `DOCKER_TLS_VERIFY` | Set this option to enable/disable Docker remote API over TLS/SSL. | 0 |
| `DOCKER_CERT_PATH` | Set this option to specify where CA cert placeholder | ~/.docker/cert.pem |

## Starting without Docker in Docker

The default way to run Fn, as it is in the Quickstart guide, is to use docker-in-docker (dind). There are
a couple reasons why we did it this way:

* It's clean. Once the container exits, there is nothing left behind including all the function images.
* You can set resource restrictions for the entire Fn instance. For instance, you can set `--memory` on the docker run command to set the max memory for the Fn instance AND all of the functions it's running.

There are some reasons you may not want to use dind, such as using the image cache during testing or you're running
[Windows](windows.md).

### Mount the Host Docker

One way is to mount the host Docker. Everything is essentially the same except you add a `-v` flag:

```sh
docker run --rm --name functions -it -v /var/run/docker.sock:/var/run/docker.sock -v $PWD/data:/app/data -p 8080:8080 fnproject/fnserver
```

On Linux systems where SELinux is enabled and set to "Enforcing", SELinux will stop the container from accessing
the host docker and the local directory mounted as a volume, so this method cannot be used unless security restrictions
are disabled.

### Run outside Docker

You can of course just run the binary directly, you'll just have to change how you set the environment variables above.

See [contributing doc](../CONTRIBUTING.md) for information on how to build and run.
