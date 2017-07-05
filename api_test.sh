set -ex


case "$1" in

    "bolt" )
    docker rm -fv func-server || echo No prev func-server container

    docker run --name func-server --privileged -v /var/run/docker.sock:/var/run/docker.sock -d -e NO_PROXY -e HTTP_PROXY -e DOCKER_HOST=${DOCKER_HOST} -e LOG_LEVEL=debug -p 8080:8080 funcy/functions
    sleep 1
    ;;

    "mysql" )
    docker rm -fv func-mysql-test || echo No prev mysql test db container
    docker rm -fv func-server || echo No prev func-server container

    docker run --name func-mysql-test -p 3307:3306 -e MYSQL_DATABASE=funcs -e MYSQL_ROOT_PASSWORD=root -d mysql
    sleep 8
    export MYSQL_HOST="$(docker inspect -f '{{.NetworkSettings.IPAddress}}' func-mysql-test)"
    export MYSQL_PORT=3306
    docker run --name func-server --privileged -d -e NO_PROXY -e HTTP_PROXY -e DOCKER_HOST=${DOCKER_HOST} -e LOG_LEVEL=debug -e "DB_URL=mysql://root:root@tcp(${MYSQL_HOST}:${MYSQL_PORT})/funcs" -p 8080:8080 -v /var/run/docker.sock:/var/run/docker.sock funcy/functions

    ;;

    "postgres" )
    docker rm -fv func-postgres-test || echo No prev test db container
    docker rm -fv func-server || echo No prev func-server container

    docker run --name func-postgres-test -p -e "POSTGRES_DB=funcs" 5432:5432 -d postgres
    sleep 8
    export POSTGRES_HOST="$(docker inspect -f '{{.NetworkSettings.IPAddress}}' func-postgres-test)"
    export POSTGRES_PORT=5432
    docker run --name func-server --privileged -d -e NO_PROXY -e HTTP_PROXY -e DOCKER_HOST=${DOCKER_HOST} -e LOG_LEVEL=debug -e "DB_URL=postgres://postgres@${POSTGRES_HOST}:${POSTGRES_PORT}/funcs?sslmode=disable" -p 8080:8080 -v /var/run/docker.sock:/var/run/docker.sock funcy/functions

    ;;

    "redis" )
    docker rm -fv func-redis-test|| echo No prev redis test db container
    docker rm -fv func-server || echo No prev func-server container

    docker run --name func-redis-test -p 6379:6379 -d redis
    sleep 8
    export REDIS_HOST="$(docker inspect -f '{{.NetworkSettings.IPAddress}}' func-redis-test)"
    export REDIS_PORT=6379
    docker run --name func-server --privileged -d -e NO_PROXY -e HTTP_PROXY -e DOCKER_HOST=${DOCKER_HOST} -e LOG_LEVEL=debug -e "DB_URL=redis://${REDIS_HOST}:${REDIS_PORT}/" -p 8080:8080 -v /var/run/docker.sock:/var/run/docker.sock funcy/functions

    ;;


esac

cd fn/tests && API_URL="http://$(docker inspect -f '{{.NetworkSettings.IPAddress}}' func-server):8080" go test -v ./...; cd ../../
