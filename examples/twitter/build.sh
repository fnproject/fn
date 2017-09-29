#!/bin/bash
set -ex

FUNCPKG=$(pwd | sed "s|$GOPATH/src/||")

# glide image to install dependencies
../build-glide.sh
docker run --rm -v "$PWD":/go/src/$FUNCPKG -w /go/src/$FUNCPKG glide up

# build image
docker run --rm -v "$PWD":/go/src/$FUNCPKG -w /go/src/$FUNCPKG fnproject/go:dev go build -o func
docker build -t username/func-twitter .
