#!/bin/bash
# Top level test script to start all other tests
set -exuo pipefail


source ./helpers.sh

remove_containers

docker run --name func-postgres-test -e "POSTGRES_DB=funcs" -e "POSTGRES_PASSWORD=root" -p 5432:5432 -d postgres:9.3-alpine
docker run --name func-mysql-test -p 3306:3306 -e MYSQL_DATABASE=funcs -e MYSQL_ROOT_PASSWORD=root -d mysql
docker run -d -p 9000:9000 --name func-minio-test -e "MINIO_ACCESS_KEY=admin" -e "MINIO_SECRET_KEY=password" minio/minio server /data

MYSQL_HOST=`host func-mysql-test`
MYSQL_PORT=3306

POSTGRES_HOST=`host func-postgres-test`
POSTGRES_PORT=5432

MINIO_HOST=`host func-minio-test`
MINIO_PORT=9000

export POSTGRES_URL="postgres://postgres:root@${POSTGRES_HOST}:${POSTGRES_PORT}/funcs?sslmode=disable"
export MYSQL_URL="mysql://root:root@tcp(${MYSQL_HOST}:${MYSQL_PORT})/funcs"
export MINIO_URL="s3://admin:password@${MINIO_HOST}:${MINIO_PORT}/us-east-1/fnlogs"

go test -v $(go list ./... | grep -v vendor | grep -v examples | grep -v test/fn-api-tests | grep -v test/fn-system-tests | grep -v images/fn-test-utils)
go vet $(go list ./... | grep -v vendor)

remove_containers

docker run -v `pwd`:/go/src/github.com/fnproject/fn --rm fnproject/swagger:0.0.1 /go/src/github.com/fnproject/fn/docs/swagger.yml
