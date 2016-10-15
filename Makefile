# Just builds

DIR := ${CURDIR}

dep:
	glide install --strip-vendor

build:
	go build -o functions

build-docker:
	set -ex
	docker run --rm -v $(DIR):/go/src/github.com/iron-io/functions -w /go/src/github.com/iron-io/functions iron/go:dev go build -o functions-alpine
	docker build -t iron/functions:latest .

test:
	go test -v $(shell glide nv | grep -v examples | grep -v tool)

test-docker:
	docker run -ti --privileged --rm -e LOG_LEVEL=debug \
	-v /var/run/docker.sock:/var/run/docker.sock \
	-v $(DIR):/go/src/github.com/iron-io/functions \
	-w /go/src/github.com/iron-io/functions iron/go:dev go test \
	-v $(shell glide nv | grep -v examples | grep -v tool)

run:
	./functions

run-docker: build-docker
	set -ex
	docker run --rm --privileged -it -e LOG_LEVEL=debug -e "DB=bolt:///app/data/bolt.db" -v $(DIR)/data:/app/data -p 8080:8080 iron/functions

all: dep build