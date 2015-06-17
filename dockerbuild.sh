#!/bin/sh


img="treeder/golang-ubuntu:1.4.2on14.04"

# note: Could use GOPATH instead to map volumes (can you do more than one -v?)
cdir=$(pwd)
godir="$(dirname "$cdir")"
echo "dir $godir"
godir="$(dirname "$godir")"
echo "dir $godir"
godir="$(dirname "$godir")"
echo "dir $godir"

# To remove a bad container and start fresh:
# docker rm router-build

# Using go install here so it installs gorocksdb the first time. It also does go build, so all good.
docker run -i --name router-build -v "$godir":/go/src -w /go/src/github.com/treeder/router -p 0.0.0.0:8080:8080 $img sh -c 'go install && go build' || docker start -ia router-build

# to bash in
#docker run -it --name ironmq-build -v "$godir":/go/src -w /go/src/github.com/iron-io/go/ironmq -p 0.0.0.0:8080:8080 $img /bin/bash
