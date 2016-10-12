#!/bin/bash

docker run -ti --privileged --rm -e LOG_LEVEL=debug -v /var/run/docker.sock:/var/run/docker.sock -v "$PWD":/go/src/github.com/iron-io/functions -w /go/src/github.com/iron-io/functions iron/go:dev go test -v $(glide nv | grep -v examples)