# Just builds
.PHONY: all test dep build test-log-datastore

dep:
	glide install -v

dep-up:
	glide up -v

build:
	go build -o fnserver

install:
	go build -o ${GOPATH}/bin/fnserver

test:
	./test.sh

test-datastore:
	cd api/datastore && go test -v ./...

test-log-datastore:
	cd api/logs && go test -v ./...

test-build-arm:
	GOARCH=arm GOARM=5 $(MAKE) build
	GOARCH=arm GOARM=6 $(MAKE) build
	GOARCH=arm GOARM=7 $(MAKE) build
	GOARCH=arm64 $(MAKE) build

run: build
	GIN_MODE=debug ./fnserver

docker-dep:
# todo: need to create a dep tool image for this (or just ditch this)
	docker run --rm -it -v ${CURDIR}:/go/src/github.com/fnproject/fn -w /go/src/github.com/fnproject/fn treeder/glide install -v

docker-build:
	docker build --build-arg HTTPS_PROXY --build-arg HTTP_PROXY -t fnproject/fnserver:latest .

docker-run: docker-build
	docker run --rm --privileged -it -e NO_PROXY -e HTTP_PROXY -e FN_LOG_LEVEL=debug -e "FN_DB_URL=sqlite3:///app/data/fn.db" -v ${CURDIR}/data:/app/data -p 8080:8080 fnproject/fnserver

docker-test-run-with-sqlite3:
	./api_test.sh sqlite3 4

docker-test-run-with-mysql:
	./api_test.sh mysql 4

docker-test-run-with-postgres:
	./api_test.sh postgres 4

docker-test:
	docker run -ti --privileged --rm -e FN_LOG_LEVEL=debug \
	-v /var/run/docker.sock:/var/run/docker.sock \
	-v ${CURDIR}:/go/src/github.com/fnproject/fn \
	-w /go/src/github.com/fnproject/fn \
	fnproject/go:dev go test \
	-v $(shell docker run --rm -ti -v ${CURDIR}:/go/src/github.com/fnproject/fn -w /go/src/github.com/fnproject/fn -e GOPATH=/go golang:alpine sh -c 'go list ./... | grep -v vendor | grep -v examples | grep -v tool | grep -v fn')

all: dep build
