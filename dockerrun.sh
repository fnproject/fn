#!/bin/sh

# If you want to start fresh, run `docker rm ironmq`

./dockerbuild.sh

img="treeder/golang-ubuntu:1.4.2on14.04"

wdir=$(pwd)
#wdir="$(dirname "$wdir")"
echo "dir $wdir"

# If you need to clean up the container, run:
#docker rm ironmq

# This creates a data container
#docker run --name ironmq-data -v /ironmq/data busybox true

docker run -it --name router -v "$wdir"/:/app -w /app -p 8080:8080 $img ./router -c config_docker.json || docker start -ia router
