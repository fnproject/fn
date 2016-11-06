#!/bin/bash
set -x

./build.sh

PAYLOAD='{"name":"Johnny"}'

# test it
echo $PAYLOAD | docker run --rm -i iron/func-hello-go