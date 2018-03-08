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
    docker rm -fv func-mysql-system-test 2>/dev/null || true
    docker rm -fv func-postgres-system-test 2>/dev/null || true
}

function wait_for_db {
  HOST="$1"
  PORT="$2"
  TIMEOUT="$3"
  for i in `seq ${TIMEOUT}` ; do
    ! nc -z "${HOST}" "${PORT}" > /dev/null 2>&1
    result=$?
    if [ $result -ne 0 ] ; then
      echo "DB listening on ${HOST}:${PORT}"
      return
    fi
    sleep 1
  done
  echo "Failed to connect to DB on ${HOST}:${PORT}"
  exit 1
}