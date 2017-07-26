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

go test -v $(go list ./... | grep -v vendor | grep -v examples | grep -v tool | grep -v fn| grep -v tmp/go/src)
# go test -v github.com/fnproject/fn/api/runner/drivers/docker

cd cli && make build && make test
# TODO: should we install fn here to use throughout?
export FN="$(pwd)/fn"
cd ..

# TODO: Test a bunch of the examples using fn test when ready
# checker tests env vars
# TODO: Fix checker tests behind proxy...
# cd examples/checker
# ./test.sh
