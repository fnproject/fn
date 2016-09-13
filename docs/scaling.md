
# Scaling IronFunctions

The QuickStart guide is intended just to quickly get started and kick the tires. To run in production and be ready to scale, there are a few more steps. 

* Run a database that can scale, such as Postgres.
* Put the iron/functions API behind a load balancer and launch more than one machine. 
* For asynchronous functions:
  * Start a separate message queue (preferably one that scales)
  * Start multiple iron/functions-runner containers, the more the merrier

There are metrics emitted to the logs that can be used to notify you when to scale. The most important being the `wait_time` metrics for both the 
synchronous and asynchronous functions. If `wait_time` increases, you'll want to start more servers with either the `iron/functions` image or the `iron/functions-runner` image. 
  
  