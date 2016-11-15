/*
IronFunctions daemon

Refer to detailed documentation at https://github.com/iron-io/functions/tree/master/docs

Environment Variables:
DB_URL
The database URL to use in URL format. See [Databases](databases/README.md) for more information. Default: BoltDB in current working directory `bolt.db`.

MQ_URL
The message queue to use in URL format. See [Message Queues](mqs/README.md) for more information. Default: BoltDB in current working directory `queue.db`.

API_URL
The primary IronFunctions API URL to that this instance will talk to. In a production environment, this would be your load balancer URL.

PORT
Sets the port to run on. Default: `8080`.

NUM_ASYNC
The number of async runners in the functions process (default 1).

LOG_LEVEL
Set to `DEBUG` to enable debugging. Default: INFO.
*/
package main
