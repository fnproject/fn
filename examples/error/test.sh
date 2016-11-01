#!/bin/bash
set -x

./build.sh

PAYLOAD='{"input": "yoooo"}'

# test it
echo $PAYLOAD | docker run --rm -i -e TEST=1 iron/func-error