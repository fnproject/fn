#!/bin/bash

HOST="${1:-localhost:8080}"
REQ="${2:-1}"

curl -H "Content-Type: application/json" -X POST -d '{
    "app": { "name":"myapp" }
}' http://$HOST/v1/apps

curl -H "Content-Type: application/json" -X POST -d '{
    "route": {
        "type": "async",
        "path":"/hello-async",
        "image":"iron/hello:ruby"
    }
}' http://$HOST/v1/apps/myapp/routes

for i in `seq 1 $REQ`;
do
    curl -H "Content-Type: application/json" -X POST -d '{
        "name":"Johnny"
    }' http://$HOST/r/myapp/hello-async
done;