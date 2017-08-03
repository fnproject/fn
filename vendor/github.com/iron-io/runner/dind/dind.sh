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
# modified from: https://github.com/docker-library/docker/blob/866c3fbd87e8eeed524fdf19ba2d63288ad49cd2/1.11/dind/dockerd-entrypoint.sh
# this will run either overlay or aufs as the docker fs driver, if the OS has both, overlay is preferred.
# rewrite overlay to use overlay2 (docker 1.12, linux >=4.x required), see https://docs.docker.com/engine/userguide/storagedriver/selectadriver/#overlay-vs-overlay2

fsdriver=$(grep -Eh -w -m1 "overlay|aufs" /proc/filesystems | cut -f2)

if [ $fsdriver == "overlay" ]; then
  fsdriver="overlay2"
fi

#https://docs.docker.com/engine/userguide/storagedriver/overlayfs-driver/#configure-docker-with-the-overlay-or-overlay2-storage-driver
sub_opt=""
case "$(uname -r)" in
  *.el7*) sub_opt="--storage-opt overlay2.override_kernel_check=1" ;;
esac

cmd="dockerd \
		--host=unix:///var/run/docker.sock \
		--host=tcp://0.0.0.0:2375 \
		--storage-driver=$fsdriver
                $sub_opt"

# nanny and restart on crashes
until eval $cmd; do
  echo "Docker crashed with exit code $?.  Respawning.." >&2
  # if we just restart it won't work, so start it (it wedges up) and
  # then kill the wedgie and restart it again and ta da... yea, seriously
  pidfile=/var/run/docker/libcontainerd/docker-containerd.pid
  kill -9 $(cat $pidfile)
  rm $pidfile
  sleep 1
done
