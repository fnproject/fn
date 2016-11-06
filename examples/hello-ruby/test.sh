#!/bin/bash
set -x

./build.sh

PAYLOAD='{"name":"Johnny"}'

# test it
echo $PAYLOAD | docker run --rm -i -e TEST=1 iron/func-hello-ruby