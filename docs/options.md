# IronFunctions Configuration Options

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
<td>DB</td>
<td>The database URL to use in URL format. See Databases below for more information. Default: BoltDB in current working directory `bolt.db`.</td>
</tr>
<tr>
<td>MQ</td>
<td>The message queue to use in URL format. See Message Queues below for more information. Default: BoltDB in current working directory `queue.db`.</td>
</tr>
<tr>
<td>API_URL</td>
<td>The primary functions api URL to pull tasks from (the address is that of another running functions process).</td>
</tr>
<tr>
<td>PORT</td>
<td>Default (8080), sets the port to run on.</td>
</tr>
<tr>
<td>NUM_ASYNC</td>
<td>The number of async runners in the functions process (default 1).</td>
</tr>
<tr>
<td>LOG_LEVEL</td>
<td>Set to `DEBUG` to enable debugging. Default is INFO.</td>
</tr>
</table>
