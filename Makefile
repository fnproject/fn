# Just builds
.PHONY: all test dep build test-log-datastore

dep:
	glide install -v

dep-up:
	glide up -v

build:
	go build -o functions

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
	GIN_MODE=debug ./functions

docker-dep:
# todo: need to create a dep tool image for this (or just ditch this)
	docker run --rm -it -v ${CURDIR}:/go/src/gitlab-odx.oracle.com/odx/functions -w /go/src/gitlab-odx.oracle.com/odx/functions treeder/glide install -v

docker-build:
	docker pull funcy/go:dev
	docker run --rm -v ${CURDIR}:/go/src/gitlab-odx.oracle.com/odx/functions -w /go/src/gitlab-odx.oracle.com/odx/functions funcy/go:dev go build -o functions-alpine
	docker build --build-arg HTTP_PROXY -t funcy/functions:latest .

docker-run: docker-build
	docker run --rm --privileged -it -e NO_PROXY -e HTTP_PROXY -e LOG_LEVEL=debug -e "DB_URL=sqlite3:///app/data/fn.db" -v ${CURDIR}/data:/app/data -p 8080:8080 funcy/functions

docker-test-run-with-sqlite3:
	./api_test.sh sqlite3

docker-test-run-with-mysql:
	./api_test.sh mysql

docker-test-run-with-postgres:
	./api_test.sh postgres

ci-docker-test-run-with-sqlite3:
	cd ${CURDIR}/test/fn-api-tests/ && docker build -t funcy/fn-api-tester .; cd ${CURDIR}
	docker run --rm -i -v ${CURDIR}:/go/src/gitlab-odx.oracle.com/odx/functions -w /go/src/gitlab-odx.oracle.com/odx/functions -e "datastore=sqlite3" -e "DOCKER_HOST=${DOCKERHOST}" funcy/fn-api-tester

docker-test:
	docker run -ti --privileged --rm -e LOG_LEVEL=debug \
	-v /var/run/docker.sock:/var/run/docker.sock \
	-v ${CURDIR}:/go/src/gitlab-odx.oracle.com/odx/functions \
	-w /go/src/gitlab-odx.oracle.com/odx/functions \
	funcy/go:dev go test \
	-v $(shell docker run --rm -ti -v ${CURDIR}:/go/src/gitlab-odx.oracle.com/odx/functions -w /go/src/gitlab-odx.oracle.com/odx/functions -e GOPATH=/go golang:alpine sh -c 'go list ./... | grep -v vendor | grep -v examples | grep -v tool | grep -v fn')

all: dep build
