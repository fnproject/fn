#!/bin/bash
set -ex

FUNCPKG=$(pwd | sed "s|$GOPATH/src/||")

# build image
docker run --rm -v "$PWD":/go/src/$FUNCPKG -w /go/src/$FUNCPKG iron/go:dev go build -o func
docker build -t iron/func-hello-go .