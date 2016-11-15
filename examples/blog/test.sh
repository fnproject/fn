#!/bin/bash
set -x

./build.sh

# test it
docker stop test-mongo-func
docker rm test-mongo-func

docker run -p 27017:27017 --name test-mongo-func -d mongo

echo '{ "title": "My New Post", "body": "Hello world!", "user": "test" }' | docker run --rm -i -e METHOD=POST -e ROUTE=/posts -e DB=mongo:27017 --link test-mongo-func:mongo -e TEST=1 iron/func-blog
docker run --rm -i -e METHOD=GET -e ROUTE=/posts -e DB=mongo:27017 --link test-mongo-func:mongo -e TEST=1 iron/func-blog

docker stop test-mongo-func
docker rm test-mongo-func