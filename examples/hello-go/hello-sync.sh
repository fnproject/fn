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


HOST="${1:-localhost:8080}"
REQ="${2:-1}"

curl -H "Content-Type: application/json" -X POST -d '{
    "app": { "name":"myapp" }
}' http://$HOST/v1/apps

curl -H "Content-Type: application/json" -X POST -d '{
    "route": {
        "type": "sync",
        "path":"/hello-sync",
        "image":"iron/hello"
    }
}' http://$HOST/v1/apps/myapp/routes

for i in `seq 1 $REQ`;
do
    curl -H "Content-Type: application/json" -X POST -d '{
        "name":"Johnny"
    }' http://$HOST/r/myapp/hello-sync
done;