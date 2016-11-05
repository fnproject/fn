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

# test it
docker stop test-mongo-func
docker rm test-mongo-func

docker run -p 27017:27017 --name test-mongo-func -d mongo

echo '{ "title": "My New Post", "body": "Hello world!", "user": "test" }' | docker run --rm -i -e METHOD=POST -e ROUTE=/posts -e CONFIG_DB=mongo:27017 --link test-mongo-func:mongo -e TEST=1 iron/func-blog  
docker run --rm -i -e METHOD=GET -e ROUTE=/posts -e CONFIG_DB=mongo:27017 --link test-mongo-func:mongo -e TEST=1 iron/func-blog

docker stop test-mongo-func
docker rm test-mongo-func