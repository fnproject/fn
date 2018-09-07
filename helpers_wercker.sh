#!/bin/bash
set -exo pipefail

function get_host {
    echo $1
}

function get_port {
    local NAME=$1

    local SERVICE_LIST=(
        "fn_basic_tests_minio"
        "fn_basic_tests_mysql"
        "fn_basic_tests_postgres"
        "fn_api_tests_minio"
        "fn_api_tests_mysql"
        "fn_api_tests_postgres"
        "fn_system_tests_minio"
        "fn_system_tests_mysql"
        "fn_system_tests_postgres"
    )

    local SERVICE_PORT_LIST=(
        9000
        3306
        5432
        9000
        3306
        5432
        9000
        3306
        5432
    )

    local IDX=0
    while [ ${IDX} -lt ${#SERVICE_LIST[@]} ]
    do
        if [ ${SERVICE_LIST[$IDX]} = "${NAME}" ]; then
            echo ${SERVICE_PORT_LIST[$IDX]}
            return
        fi
        IDX=$(($IDX+1))
    done

    echo "Invalid context/component: ${NAME} not in service list"
    exit -1
}

function spawn_sqlite3 {
    local CONTEXT=$1
    touch /tmp/${CONTEXT}_sqllite3.db
    echo "sqlite3:///tmp/${CONTEXT}_sqllite3.db"
}

function spawn_mysql {
    local CONTEXT=$1
    local PORT=$(get_port ${CONTEXT}_mysql)
    local HOST=$(get_host ${CONTEXT}_mysql)
    local ID=$(docker run --name ${CONTEXT}_mysql \
        -p ${PORT}:3306 \
        -e MYSQL_DATABASE=funcs \
        -e MYSQL_ROOT_PASSWORD=root \
        --network=$DOCKER_NETWORK_NAME \
        -d mysql:5.7.22)

    echo "mysql://root:root@tcp(${HOST}:${PORT})/funcs"
}

function spawn_postgres {
    local CONTEXT=$1
    local PORT=$(get_port ${CONTEXT}_postgres)
    local HOST=$(get_host ${CONTEXT}_postgres)
    local ID=$(docker run --name ${CONTEXT}_postgres \
        -e "POSTGRES_DB=funcs" \
        -e "POSTGRES_PASSWORD=root" \
        -p ${PORT}:5432 \
        --network=$DOCKER_NETWORK_NAME \
        -d postgres:9.3-alpine)

    echo "postgres://postgres:root@${HOST}:${PORT}/funcs?sslmode=disable"
}

function spawn_minio {
    local CONTEXT=$1
    local PORT=$(get_port ${CONTEXT}_minio)
    local HOST=$(get_host ${CONTEXT}_minio)
    local ID=$(docker run --name ${CONTEXT}_minio \
        -p ${PORT}:9000 \
        -e "MINIO_ACCESS_KEY=admin" \
        -e "MINIO_SECRET_KEY=password" \
        --network=$DOCKER_NETWORK_NAME \
        -d minio/minio server /data)

    echo "s3://admin:password@${HOST}:${PORT}/us-east-1/fnlogs"
}

function docker_pull_postgres {
	docker pull postgres:9.3-alpine
}

function docker_pull_mysql {
	docker pull mysql:5.7.22
}

function docker_pull_minio {
	docker pull minio/minio
}

function remove_containers {
    local CONTEXT=$1
    for i in mysql minio postgres
    do
        docker rm -fv ${CONTEXT}_${i} 2>/dev/null || true
    done

    rm -f /tmp/${CONTEXT}_sqllite3.db
}
