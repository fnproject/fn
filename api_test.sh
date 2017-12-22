#!/bin/bash
set -exuo pipefail

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
    FN_DB_URL="sqlite3:///tmp/fn_integration_tests.db"
    ;;

    "mysql" )
    DB_CONTAINER="func-mysql-test"
    docker rm -fv ${DB_CONTAINER} || echo No prev mysql test db container
    docker run --name ${DB_CONTAINER} -p 3306:3306 -e MYSQL_DATABASE=funcs -e MYSQL_ROOT_PASSWORD=root -d mysql
    sleep 5
    MYSQL_HOST=`host ${DB_CONTAINER}`
    MYSQL_PORT=3306
    FN_DB_URL="mysql://root:root@tcp(${MYSQL_HOST}:${MYSQL_PORT})/funcs"

    ;;

    "postgres" )
    DB_CONTAINER="func-postgres-test"
    docker rm -fv ${DB_CONTAINER} || echo No prev test db container
    docker run --name ${DB_CONTAINER} -e "POSTGRES_DB=funcs" -e "POSTGRES_PASSWORD=root"  -p 5432:5432 -d postgres
    sleep 5
    POSTGRES_HOST=`host ${DB_CONTAINER}`
    POSTGRES_PORT=5432
    FN_DB_URL="postgres://postgres:root@${POSTGRES_HOST}:${POSTGRES_PORT}/funcs?sslmode=disable"

    ;;
esac

cd test/fn-api-tests && FN_API_URL="http://localhost:8080"  FN_DB_URL=${FN_DB_URL} go test -v  -parallel ${2:-1} ./...; cd ../../
