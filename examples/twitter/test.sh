#!/bin/bash
set -x

./build.sh

PAYLOAD='{"username": "getiron"}'

# test it
echo $PAYLOAD | docker run --rm -i iron/func-twitter 