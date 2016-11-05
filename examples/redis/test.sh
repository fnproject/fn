#!/bin/bash

# Copyright 2016 Iron.io
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

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

echo $PAYLOAD | docker run --rm -i -e CONFIG_SERVER=redis:6379 -e CONFIG_COMMAND=SET --link test-redis-func:redis iron/func-redis
echo $PAYLOAD | docker run --rm -i -e CONFIG_SERVER=redis:6379 -e CONFIG_COMMAND=GET --link test-redis-func:redis iron/func-redis

docker stop test-redis-func
docker rm test-redis-func