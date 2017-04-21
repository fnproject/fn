#!/bin/sh

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

set -ex

sleep 600 # 10 minutes

for i in 1..1000; do
  pkill -9 dockerd
  pkill -9 docker-containerd
  # remove pid file since we killed docker hard
  rm /var/run/docker.pid
  sleep 30
  docker daemon \
      --host=unix:///var/run/docker.sock \
      --host=tcp://0.0.0.0:2375 &
  sleep 300 # 5 minutes
done
