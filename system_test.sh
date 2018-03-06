#!/bin/bash
set -exo pipefail

source ./helpers.sh

remove_containers

case "$1" in
    "sqlite3" )
    rm -fr /tmp/fn_system_tests.db
    touch /tmp/fn_system_tests.db
    export FN_DB_URL="sqlite3:///tmp/fn_system_tests.db"
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
    docker run --name ${DB_CONTAINER} -e "POSTGRES_DB=funcs" -e "POSTGRES_PASSWORD=root"  -p 5432:5432 -d postgres:9.3-alpine
    POSTGRES_HOST=`host ${DB_CONTAINER}`
    POSTGRES_PORT=5432
    export FN_DB_URL="postgres://postgres:root@${POSTGRES_HOST}:${POSTGRES_PORT}/funcs?sslmode=disable"

    ;;
esac

cd test/fn-system-tests && FN_DB_URL=${FN_DB_URL} go test -v  -parallel ${2:-1} ./...; cd ../../

remove_containers