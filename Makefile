# Just builds
.PHONY: all test dep build test-log-datastore checkfmt pull-images api-test fn-test-utils test-middleware test-extensions test-basic test-api

dep:
	dep ensure --vendor-only

dep-up:
	dep ensure

build:
	go build -o fnserver

install:
	go build -o ${GOPATH}/bin/fnserver

checkfmt:
	./go-fmt.sh

clear-images:
	-docker images -q -f dangling=true | xargs docker rmi -f
	for i in fnproject/fn-test-utils fnproject/hello fnproject/sleeper \
	         fnproject/dind fnproject/fnserver fnproject/fnlb; do \
	    docker images "$$i" --format '{{ .ID }}\t{{ .Repository }}\t{{ .Tag}}' | while read id repo tag; do \
	        if [ "$$tag" = "<none>" ]; then docker rmi "$$id"; else docker rmi "$$repo:$$tag"; fi; done; done

release-fnserver:
	./release.sh

build-dind:
	(cd images/dind && ./build.sh)

release-dind:
	(cd images/dind && ./release.sh)

fn-test-utils: checkfmt
	cd images/fn-test-utils && ./build.sh

test-middleware: test-basic
	cd examples/middleware && go build

test-extensions: test-basic
	cd examples/extensions && go build

test-basic: checkfmt pull-images fn-test-utils
	./test.sh

test: checkfmt pull-images test-basic test-middleware test-extensions

test-api: test-basic
	./api_test.sh sqlite3 4
	./api_test.sh mysql 4 0
	./api_test.sh postgres 4 0

build-static:
	go install

full-test: build-static test test-api

img-sleeper:
	docker pull fnproject/sleeper
img-hello:
	docker pull fnproject/hello
img-mysql:
	docker pull mysql
img-postgres:
	docker pull postgres:9.3-alpine
img-minio:
	docker pull minio/minio

pull-images: img-sleeper img-hello img-mysql img-postgres img-minio

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
