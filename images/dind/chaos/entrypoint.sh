#!/bin/sh

set -ex

# modified from: https://github.com/docker-library/docker/blob/866c3fbd87e8eeed524fdf19ba2d63288ad49cd2/1.11/dind/dockerd-entrypoint.sh
# this will run either overlay or aufs as the docker fs driver, if the OS has both, overlay is preferred.

docker daemon \
		--host=unix:///var/run/docker.sock \
		--host=tcp://0.0.0.0:2375 &

# wait for daemon to initialize
sleep 10

/usr/local/bin/chaos.sh &

exec "$@"
