#!/bin/bash
set -x

./build.sh

PAYLOAD='{"sleep": 5}' 

# test it
echo $PAYLOAD | docker run --rm -i -e TEST=1 iron/func-sleeper