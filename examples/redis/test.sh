#!/bin/bash
set -x

./build.sh

PAYLOAD='{
    "key": "test",
    "value": "123"
}'

# test it
docker stop test-redis-func
docker rm test-redis-func

docker run -p 6379:6379 --name test-redis-func -d redis

echo $PAYLOAD | docker run --rm -i -e SERVER=redis:6379 -e COMMAND=SET --link test-redis-func:redis iron/func-redis
echo $PAYLOAD | docker run --rm -i -e SERVER=redis:6379 -e COMMAND=GET --link test-redis-func:redis iron/func-redis

docker stop test-redis-func
docker rm test-redis-func