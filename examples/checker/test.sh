#!/bin/bash
set -x

./build.sh

PAYLOAD='{"env_vars": {"FOO": "bar"}}'

# test it
echo $PAYLOAD | docker run --rm -i -e TEST=1 -e FOO=bar username/func-checker