# IronFunctions Runtime Options

## Configuration

When starting IronFunctions, you can pass in the following configuration variables as environment variables. Use `-e VAR_NAME=VALUE` in
docker run.  For example:

```
docker run -e VAR_NAME=VALUE ...
```

<table>
<tr>
<th>Env Variables</th>
<th>Description</th>
</tr>
<tr>
<td>DB_URL</td>
<td>The database URL to use in URL format. See [Databases](databases/README.md) for more information. Default: BoltDB in current working directory `bolt.db`.</td>
</tr>
<tr>
<td>MQ_URL</td>
<td>The message queue to use in URL format. See [Message Queues](mqs/README.md) for more information. Default: BoltDB in current working directory `queue.db`.</td>
</tr>
<tr>
<td>API_URL</td>
<td>The primary IronFunctions API URL to that this instance will talk to. In a production environment, this would be your load balancer URL.</td>
</tr>
<tr>
<td>PORT</td>
<td>Sets the port to run on. Default: `8080`.</td>
</tr>
<tr>
<td>LOG_LEVEL</td>
<td>Set to `DEBUG` to enable debugging. Default: INFO.</td>
</tr>
</table>

## Starting without Docker in Docker

The default way to run IronFunctions, as it is in the Quickstart guide, is to use docker-in-docker (dind). There are
a couple reasons why we did it this way:

* It's clean. Once the container exits, there is nothing left behind including all the function images.
* You can set resource restrictions for the entire IronFunctions instance. For instance, you can set `--memory` on
the docker run command to set the max memory for the IronFunctions instance AND all of the functions it's running.

There are some reasons you may not want to use dind, such as using the image cache during testing or you're running
[Windows](windows.md).

### Mount the Host Docker

One way is to mount the host Docker. Everything is essentially the same except you add a `-v` flag:

```sh
docker run --rm --name functions -it -v /var/run/docker.sock:/var/run/docker.sock -v $PWD/data:/app/data -p 8080:8080 iron/functions
```

### Run outside Docker

You can of course just run the binary directly, you'll just have to change how you set the environment variables above.

See [contributing doc](../CONTRIBUTING.md) for information on how to build and run.

