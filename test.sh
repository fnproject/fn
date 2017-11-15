#!/bin/bash
# Top level test script to start all other tests

set -ex

docker rm -fv func-postgres-test || echo No prev test db container
docker run --name func-postgres-test -p 15432:5432 -d postgres
docker rm -fv func-mysql-test || echo No prev mysql test db container
docker run --name func-mysql-test -p 3307:3306 -e MYSQL_DATABASE=funcs -e MYSQL_ROOT_PASSWORD=root -d mysql
sleep 5
case ${DOCKER_LOCATION:-localhost} in
localhost)
    export POSTGRES_HOST=localhost
    export POSTGRES_PORT=15432

    export MYSQL_HOST=localhost
    export MYSQL_PORT=3307
    ;;
docker_ip)
    if [[ !  -z  ${DOCKER_HOST}  ]]
    then
        DOCKER_IP=`echo ${DOCKER_HOST} | awk -F/ '{print $3}'| awk -F: '{print $1}'`
    fi
    export POSTGRES_HOST=${DOCKER_IP:-localhost}
    export POSTGRES_PORT=15432

    export MYSQL_HOST=${DOCKER_IP:-localhost}
    export MYSQL_PORT=3307
    ;;
container_ip)
    export POSTGRES_HOST="$(docker inspect -f '{{.NetworkSettings.IPAddress}}' func-postgres-test)"
    export POSTGRES_PORT=5432

    export MYSQL_HOST="$(docker inspect -f '{{.NetworkSettings.IPAddress}}' func-mysql-test)"
    export MYSQL_PORT=3306
    ;;
esac

go test -v $(go list ./... | grep -v vendor | grep -v examples | grep -v test/fn-api-tests)
go vet -v $(go list ./... | grep -v vendor)
docker rm --force func-postgres-test
docker rm --force func-mysql-test

docker run -v `pwd`:/go/src/github.com/fnproject/fn --rm  quay.io/goswagger/swagger validate /go/src/github.com/fnproject/fn/docs/swagger.yml

# test middlware, extensions, examples, etc
# TODO: do more here, maybe as part of fn tests
cd examples/middleware
go build
cd ../..
cd examples/extensions
go build
cd ../..
