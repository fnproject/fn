#!/bin/bash
set -exo pipefail

function host {
    case ${DOCKER_LOCATION:-localhost} in
    localhost)
        echo "localhost"
        ;;
    docker_ip)
        if [[ !  -z  ${DOCKER_HOST}  ]]
        then
            DOCKER_IP=`echo ${DOCKER_HOST} | awk -F/ '{print $3}'| awk -F: '{print $1}'`
        fi

        echo ${DOCKER_IP}
        ;;
    container_ip)
        echo "$(docker inspect -f '{{.NetworkSettings.IPAddress}}' ${1})"
        ;;
    esac
}

case "$1" in
    "sqlite3" )
    rm -fr /tmp/fn_integration_tests.db
    touch /tmp/fn_integration_tests.db
    export FN_DB_URL="sqlite3:///tmp/fn_integration_tests.db"
    ;;

    "mysql" )
    DB_CONTAINER="func-mysql-test"
    docker rm -fv ${DB_CONTAINER} || echo No prev mysql test db container
    docker run --name ${DB_CONTAINER} -p 3306:3306 -e MYSQL_DATABASE=funcs -e MYSQL_ROOT_PASSWORD=root -d mysql
    MYSQL_HOST=`host ${DB_CONTAINER}`
    MYSQL_PORT=3306
    export FN_DB_URL="mysql://root:root@tcp(${MYSQL_HOST}:${MYSQL_PORT})/funcs"

    ;;

    "postgres" )
    DB_CONTAINER="func-postgres-test"
    docker rm -fv ${DB_CONTAINER} || echo No prev test db container
    docker run --name ${DB_CONTAINER} -e "POSTGRES_DB=funcs" -e "POSTGRES_PASSWORD=root"  -p 5432:5432 -d postgres
    POSTGRES_HOST=`host ${DB_CONTAINER}`
    POSTGRES_PORT=5432
    export FN_DB_URL="postgres://postgres:root@${POSTGRES_HOST}:${POSTGRES_PORT}/funcs?sslmode=disable"

    ;;
esac

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
cd test/fn-api-tests && FN_API_URL="http://localhost:8080"  FN_DB_URL=${FN_DB_URL} go test -v  -parallel ${2:-1} ./...; cd ../../
