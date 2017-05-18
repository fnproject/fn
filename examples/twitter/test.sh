#!/bin/bash
set -x

./build.sh

PAYLOAD='{"username": "joe"}'

# test it
echo $PAYLOAD | docker run --rm -i username/func-twitter 