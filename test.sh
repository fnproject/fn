#!/bin/bash
# Top level test script to start all other tests
set -exuo pipefail

export CONTEXT="fn_basic_tests"
source ./helpers.sh
remove_containers ${CONTEXT}

export GOFLAGS=-mod=vendor
export POSTGRES_URL=$(spawn_postgres ${CONTEXT})
export MYSQL_URL=$(spawn_mysql ${CONTEXT})
export MINIO_URL=$(spawn_minio ${CONTEXT})
export FN_DS_DB_PING_MAX_RETRIES=60

go test $(go list ./... | \
    grep -v vendor | \
    grep -v examples | \
    grep -v test/fn-api-tests | \
    grep -v test/fn-system-tests | \
    grep -v images/fn-test-utils\
)

go vet $(go list ./... | grep -v vendor)

remove_containers ${CONTEXT}

docker run -v `pwd`:/go/src/github.com/fnproject/fn --rm fnproject/swagger:0.0.1 /go/src/github.com/fnproject/fn/docs/swagger_v2.yml
