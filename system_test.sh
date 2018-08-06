#!/bin/bash
set -exuo pipefail

export CONTEXT="fn_system_tests"
source ./helpers.sh
remove_containers ${CONTEXT}

DB_NAME=$1
export FN_DB_URL=$(spawn_${DB_NAME} ${CONTEXT})

# avoid port conflicts with api_test.sh which are run in parallel
export FN_API_URL="http://localhost:8085"
export FN_DS_DB_PING_MAX_RETRIES=60

# pure runner and LB agent required settings below
export FN_MAX_RESPONSE_SIZE=6291456
export FN_ENABLE_NB_RESOURCE_TRACKER=1

#
# dump prometheus metrics to this file
#
export SYSTEM_TEST_PROMETHEUS_FILE=./prometheus.${DB_NAME}.txt

cd test/fn-system-tests
go test -v ./...
cd ../../

remove_containers ${CONTEXT}
