#!/bin/bash
set -exo pipefail

function get_host {
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

function get_port {
    local NAME=$1
    local PORT_START=${FN_TEST_PORT_RANGE_START:-33000}

    local SERVICE_LIST=(
        "fn_basic_tests_mysql"
        "fn_basic_tests_postgres"
        "fn_api_tests_mysql"
        "fn_api_tests_postgres"
        "fn_system_tests_mysql"
        "fn_system_tests_postgres"
    )

    local IDX=0
    while [ ${IDX} -lt ${#SERVICE_LIST[@]} ]
    do
        if [ ${SERVICE_LIST[$IDX]} = "${NAME}" ]; then
            echo $((${PORT_START}+${IDX}))
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
        -d postgres:9.3-alpine)

    echo "postgres://postgres:root@${HOST}:${PORT}/funcs?sslmode=disable"
}

function docker_pull_postgres {
	docker pull postgres:9.3-alpine
}

function docker_pull_mysql {
	docker pull mysql:5.7.22
}

function remove_containers {
    local CONTEXT=$1
    for i in mysql postgres
    do
        docker rm -fv ${CONTEXT}_${i} 2>/dev/null || true
    done

    rm -f /tmp/${CONTEXT}_sqllite3.db
}
