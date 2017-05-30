#!/bin/bash
set -ex

PAYLOAD='{"env_vars": {"FOO": "bar"}}'

# test it
: ${FN:="fn"}
echo $PAYLOAD | $FN run -e FOO=bar
