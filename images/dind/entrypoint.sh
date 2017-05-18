#!/bin/sh

set -ex

/usr/local/bin/dind.sh &

# wait for daemon to initialize
sleep 3

exec "$@"
