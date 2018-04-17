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

function remove_containers {
    docker rm -fv func-postgres-test 2>/dev/null || true
    docker rm -fv func-mysql-test 2>/dev/null || true
    docker rm -fv func-minio-test 2>/dev/null || true
}
