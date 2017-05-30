# Just builds
.PHONY: all test dep build

dep:
	dep ensure

build:
	go build -o functions

test:
	go test -v $(shell go list ./... | grep -v vendor | grep -v examples | grep -v tool | grep -v fn)
	cd fn && $(MAKE) test

test-datastore:
	cd api/datastore && go test -v ./...

test-build-arm:
    GOARCH=arm GOARM=5 $(MAKE) build
    GOARCH=arm GOARM=6 $(MAKE) build
    GOARCH=arm GOARM=7 $(MAKE) build
    GOARCH=arm64 $(MAKE) build

run: build
	GIN_MODE=debug ./functions

docker-dep:
# todo: need to create a dep tool image for this (or just ditch this)
	docker run --rm -it -v ${CURDIR}:/go/src/gitlab-odx.oracle.com/odx/functions -w /go/src/gitlab-odx.oracle.com/odx/functions treeder/glide install -v

docker-build:
	docker pull funcy/go:dev
	docker run --rm -v ${CURDIR}:/go/src/gitlab-odx.oracle.com/odx/functions -w /go/src/gitlab-odx.oracle.com/odx/functions funcy/go:dev go build -o functions-alpine
	docker build --build-arg HTTP_PROXY -t treeder/functions:latest .

docker-run: docker-build
	docker run --rm --privileged -it -e LOG_LEVEL=debug -e "DB_URL=bolt:///app/data/bolt.db" -v ${CURDIR}/data:/app/data -p 8080:8080 treeder/functions

docker-test:
	docker run -ti --privileged --rm -e LOG_LEVEL=debug \
	-v /var/run/docker.sock:/var/run/docker.sock \
	-v ${CURDIR}:/go/src/gitlab-odx.oracle.com/odx/functions \
	-w /go/src/gitlab-odx.oracle.com/odx/functions \
	funcy/go:dev go test \
	-v $(shell docker run -ti -v ${CURDIR}:/go/src/gitlab-odx.oracle.com/odx/functions -w /go/src/gitlab-odx.oracle.com/odx/functions -e GOPATH=/go golang:alpine sh -c 'go list ./... | grep -v vendor | grep -v examples | grep -v tool | grep -v fn | grep -v datastore')

all: dep build
