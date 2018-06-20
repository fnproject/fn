#!/bin/bash
set -exo pipefail

export CONTEXT="fn_api_tests"
source ./helpers.sh
remove_containers ${CONTEXT}

DB_NAME=$1
export FN_DB_URL=$(spawn_${DB_NAME} ${CONTEXT})

#test test/fn-api-tests/fn-api-tests.test
#status=`echo $?`
#rebuild="${3:-1}"
#circleci=`echo ${CIRCLECI}`
#cd test/fn-api-tests
#if [[ $status -ne 0 ]] || [[ $rebuild -ne 0 ]] ; then
#    if [[ $circleci == "true" ]]; then
#        # dirty things to make CI pass
#        ls -lah /usr/local/go/pkg/linux_amd64/runtime
#        sudo chown -R `whoami`:root /usr/local/go
#        sudo chmod -R 777 /usr/local/go/pkg/linux_amd64
#        ls -lah /usr/local/go/pkg/linux_amd64/runtime
#    fi
#    pwd
#    go test -i -a -o fn-api-tests.test
#fi
#pwd
#./fn-api-tests.test -test.v  -test.parallel ${2:-1} ./...; cd ../../

export FN_DS_DB_PING_MAX_RETRIES=60
cd test/fn-api-tests && FN_API_URL="http://localhost:8080"  FN_DB_URL=${FN_DB_URL} go test -v  -parallel ${2:-1} ./...; cd ../../

remove_containers ${CONTEXT}
