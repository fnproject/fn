# Just builds
.PHONY: all 

.PHONY: dep
dep:
	dep ensure --vendor-only

.PHONY: dep-up
dep-up:
	dep ensure

.PHONY: build
build: api/agent/grpc/runner.pb.go
	go build -o fnserver ./cmd/fnserver 

.PHONY: install
install:
	go build -o ${GOPATH}/bin/fnserver ./cmd/fnserver 

.PHONY: checkfmt
checkfmt:
	./go-fmt.sh

.PHONY: clear-images
clear-images:
	-docker images -q -f dangling=true | xargs docker rmi -f
	for i in fnproject/fn-test-utils fnproject/hello fnproject/dind fnproject/fnserver ; do \
	    docker images "$$i" --format '{{ .ID }}\t{{ .Repository }}\t{{ .Tag}}' | while read id repo tag; do \
	        if [ "$$tag" = "<none>" ]; then docker rmi "$$id"; else docker rmi "$$repo:$$tag"; fi; done; done

.PHONY: release-all
release-all: release-dind release-fn-test-utils release-fn

.PHONY: build-all
build-all: full-test build-dind build-fn-test-utils

.PHONY: build-dind
build-dind:
	(cd images/dind && ./build.sh)

.PHONY: release-dind
release-dind:
	(cd images/dind && ./release.sh)

.PHONY: release-fn-test-utils
release-fn-test-utils:
	(cd images/fn-test-utils && ./release.sh)

.PHONY: build-fn-test-utils
build-fn-test-utils: checkfmt
	cd images/fn-test-utils && ./build.sh

.PHONY: release-fn
release-fn: docker-build
	./release.sh

.PHONY: test-middleware
test-middleware: test-basic
	cd examples/middleware && go build

.PHONY: test-extensions
test-extensions: test-basic
	cd examples/extensions && go build

.PHONY: test-basic
test-basic: checkfmt pull-images build-fn-test-utils
	./test.sh

.PHONY: test
test: checkfmt pull-images test-basic test-middleware test-extensions

.PHONY: test-api
test-api: test-basic
	./api_test.sh sqlite3 4
	./api_test.sh mysql 4 0
	./api_test.sh postgres 4 0

.PHONY: test-system
test-system: test-basic
	./system_test.sh sqlite3 4
	./system_test.sh mysql 4 0
	./system_test.sh postgres 4 0

.PHONY: full-test
full-test: test test-api test-system

.PHONY: img-busybox
img-busybox:
	docker pull busybox

.PHONY: img-hello
img-hello:
	docker pull fnproject/hello

.PHONY: img-mysql
img-mysql:
	docker pull mysql

.PHONY: img-postgres
img-postgres:
	docker pull postgres:9.3-alpine

.PHONY: img-minio
img-minio:
	docker pull minio/minio

.PHONY: pull-images
pull-images: img-hello img-mysql img-postgres img-minio img-busybox

.PHONY: test-datastore
test-datastore:
	cd api/datastore && go test -v ./...

.PHONY: test-log-datastore
test-log-datastore:
	cd api/logs && go test -v ./...

.PHONY: test-build-arm
test-build-arm:
	GOARCH=arm GOARM=5 $(MAKE) build
	GOARCH=arm GOARM=6 $(MAKE) build
	GOARCH=arm GOARM=7 $(MAKE) build
	GOARCH=arm64 $(MAKE) build

%.pb.go: %.proto
	protoc --proto_path=$(@D) --proto_path=./vendor --go_out=plugins=grpc:$(@D) $<

.PHONY: run
run: build
	GIN_MODE=debug ./fnserver

.PHONY: docker-build
docker-build:
	docker build --build-arg HTTPS_PROXY --build-arg HTTP_PROXY -t fnproject/fnserver:latest .

.PHONY: docker-run
docker-run: docker-build
	docker run --rm --privileged -it -e NO_PROXY -e HTTP_PROXY -e FN_LOG_LEVEL=debug -e "FN_DB_URL=sqlite3:///app/data/fn.db" -v ${CURDIR}/data:/app/data -p 8080:8080 fnproject/fnserver

.PHONY: docker-test-run-with-sqlite3
docker-test-run-with-sqlite3:
	./api_test.sh sqlite3 4

.PHONY: docker-test-run-with-mysql
docker-test-run-with-mysql:
	./api_test.sh mysql 4

.PHONY: docker-test-run-with-postgres
docker-test-run-with-postgres:
	./api_test.sh postgres 4

.PHONY: docker-test
docker-test:
	docker run -ti --privileged --rm -e FN_LOG_LEVEL=debug \
	-v /var/run/docker.sock:/var/run/docker.sock \
	-v ${CURDIR}:/go/src/github.com/fnproject/fn \
	-w /go/src/github.com/fnproject/fn \
	fnproject/go:dev go test \
	-v $(shell docker run --rm -ti -v ${CURDIR}:/go/src/github.com/fnproject/fn -w /go/src/github.com/fnproject/fn -e GOPATH=/go golang:alpine sh -c 'go list ./... | grep -v vendor | grep -v examples | grep -v tool | grep -v fn')

all: dep build
