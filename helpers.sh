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

function install_swagger_tool {
    if [[ ! -f ${GOPATH}/bin/swagger ]]; then
        case "$(uname)" in
          Linux)
            curl -L https://github.com/go-swagger/go-swagger/releases/download/0.13.0/swagger_linux_amd64 -o ./swagger
          ;;
          Darwin)
            curl -L https://github.com/go-swagger/go-swagger/releases/download/0.13.0/swagger_darwin_amd64 -o ./swagger
        esac
    fi
    chmod +x ./swagger
}
