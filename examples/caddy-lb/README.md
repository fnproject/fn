# Fn Load Balance example using Caddy

Simple example of Fn load balancer using Caddy Server


## Run Fn

Start the Fn instances

Ref: https://github.com/fnproject/functions/blob/master/README.md#start-the-functions-api


## Configure environment variable

Pass the host and port of Fn instances in environment variables, 
this example uses three Fn instances.

```sh
export LB_HOST01="172.17.0.1:8080"
export LB_HOST02="172.17.0.1:8081"
export LB_HOST03="172.17.0.1:8082"
```

Note: Caddy doesn't support multiple hosts in only one variable. 


## Run Caddy

```sh
docker run --rm  \
    -v $PWD/Caddyfile:/etc/Caddyfile  \
    -e LB_HOST01=$LB_HOST01 -e LB_HOST02=$LB_HOST02 -e LB_HOST03=$LB_HOST03 \
    -p 9000:9000  \
    abiosoft/caddy
```

## Execute a function

Follow the Quick-Start steps replacing the example hosts by the Caddy host (localhost:9000)

https://github.com/fnproject/functions/blob/master/README.md#quick-start


## Docker Compose example

This is an additional example.

```sh
docker-compose up
```


## Caddy Reference: 

* https://github.com/mholt/caddy
* https://caddyserver.com/


