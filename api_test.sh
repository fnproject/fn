set -ex

case "$1" in
    "sqlite3" )
    # docker rm -fv func-server || echo No prev func-server container
    #
    # docker run --name func-server --privileged -v /var/run/docker.sock:/var/run/docker.sock -d -e NO_PROXY -e HTTP_PROXY -e DOCKER_HOST=${DOCKER_HOST} -e LOG_LEVEL=debug -p 8080:8080 funcy/functions
    # sleep 10
    # docker logs func-server
    # docker inspect -f '{{.NetworkSettings.IPAddress}}' func-server
    ;;

    "mysql" )
    docker rm -fv func-mysql-test || echo No prev mysql test db container
    docker rm -fv func-server || echo No prev func-server container

    docker run --name func-mysql-test -p 3306:3306 -e MYSQL_DATABASE=funcs -e MYSQL_ROOT_PASSWORD=root -d mysql
    sleep 30
    docker logs func-mysql-test
    export MYSQL_HOST="$(docker inspect -f '{{.NetworkSettings.IPAddress}}' func-mysql-test)"
    export MYSQL_PORT=3306
    docker run --name func-server --privileged -d -e NO_PROXY -e HTTP_PROXY -e DOCKER_HOST=${DOCKER_HOST} -e LOG_LEVEL=debug -e "DB_URL=mysql://root:root@tcp(${MYSQL_HOST}:${MYSQL_PORT})/funcs" -p 8080:8080 -v /var/run/docker.sock:/var/run/docker.sock funcy/functions
    docker logs func-server
    docker inspect -f '{{.NetworkSettings.IPAddress}}' func-mysql-test
    docker inspect -f '{{.NetworkSettings.IPAddress}}' func-server

    ;;

    "postgres" )
    docker rm -fv func-postgres-test || echo No prev test db container
    docker rm -fv func-server || echo No prev func-server container

    docker run --name func-postgres-test -e "POSTGRES_DB=funcs" -e "POSTGRES_PASSWORD=root"  -p 5432:5432 -d postgres
    sleep 30
    docker logs func-postgres-test
    export POSTGRES_HOST="$(docker inspect -f '{{.NetworkSettings.IPAddress}}' func-postgres-test)"
    export POSTGRES_PORT=5432
    docker run --name func-server --privileged -d -e NO_PROXY -e HTTP_PROXY -e DOCKER_HOST=${DOCKER_HOST} -e LOG_LEVEL=debug -e "DB_URL=postgres://postgres:root@${POSTGRES_HOST}:${POSTGRES_PORT}/funcs?sslmode=disable" -p 8080:8080 -v /var/run/docker.sock:/var/run/docker.sock funcy/functions
    docker logs func-server
    docker inspect -f '{{.NetworkSettings.IPAddress}}' func-postgres-test
    docker inspect -f '{{.NetworkSettings.IPAddress}}' func-server

    ;;
esac

case ${DOCKER_LOCATION:-localhost} in
localhost)
    cd test/fn-api-tests && API_URL="http://localhost:8080" go test -v ./...; cd ../../
    ;;
docker_ip)
    if [[ !  -z  ${DOCKER_HOST}  ]]
    then
        DOCKER_IP=`echo ${DOCKER_HOST} | awk -F/ '{print $3}'| awk -F: '{print $1}'`
    fi

    cd test/fn-api-tests && API_URL="http://${DOCKER_IP:-localhost}:8080" go test -v ./...; cd ../../
    ;;
container_ip)
    cd test/fn-api-tests && API_URL="http://"$(docker inspect -f '{{.NetworkSettings.IPAddress}}' func-server)":8080" go test -v ./...; cd ../../
    ;;
esac
