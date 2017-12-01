#!/bin/bash
# Top level test script to start all other tests

set -ex

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

docker rm -fv func-postgres-test || echo No prev test db container
docker run --name func-postgres-test -e "POSTGRES_DB=funcs" -e "POSTGRES_PASSWORD=root" -p 5432:5432 -d postgres
docker rm -fv func-mysql-test || echo No prev mysql test db container
docker run --name func-mysql-test -p 3306:3306 -e MYSQL_DATABASE=funcs -e MYSQL_ROOT_PASSWORD=root -d mysql
docker rm -fv func-minio-test || echo No prev minio test container
docker run -d -p 9000:9000 --name func-minio-test -e "MINIO_ACCESS_KEY=admin" -e "MINIO_SECRET_KEY=password" minio/minio server /data

# pull all images used in tests so that tests don't time out and fail spuriously
docker pull fnproject/sleeper
docker pull fnproject/error
docker pull fnproject/hello

MYSQL_HOST=`host func-mysql-test`
MYSQL_PORT=3306

POSTGRES_HOST=`host func-postgres-test`
POSTGRES_PORT=5432

MINIO_HOST=`host func-minio-test`
MINIO_PORT=9000

export POSTGRES_URL="postgres://postgres:root@${POSTGRES_HOST}:${POSTGRES_PORT}/funcs?sslmode=disable"
export MYSQL_URL="mysql://root:root@tcp(${MYSQL_HOST}:${MYSQL_PORT})/funcs"
export MINIO_URL="s3://admin:password@${MINIO_HOST}:${MINIO_PORT}/us-east-1/fnlogs"

go test -v $(go list ./... | grep -v vendor | grep -v examples | grep -v test/fn-api-tests)
go vet -v $(go list ./... | grep -v vendor)
docker rm --force func-postgres-test
docker rm --force func-mysql-test
docker rm --force func-minio-test

docker run -v `pwd`:/go/src/github.com/fnproject/fn --rm  quay.io/goswagger/swagger validate /go/src/github.com/fnproject/fn/docs/swagger.yml

# test middlware, extensions, examples, etc
# TODO: do more here, maybe as part of fn tests
cd examples/middleware
go build
cd ../..
cd examples/extensions
go build
cd ../..
