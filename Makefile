# Just builds
.PHONY: dep
dep:
	dep ensure --vendor-only

.PHONY: dep-up
dep-up:
	dep ensure

.PHONY: build
build:
	go build -o fnserver ./cmd/fnserver 

.PHONY: generate
generate: api/agent/grpc/runner.pb.go

.PHONY: install
install:
	go build -o ${GOPATH}/bin/fnserver ./cmd/fnserver 

.PHONY: checkfmt
checkfmt:
	./go-fmt.sh

.PHONY: clear-images
clear-images:
	-docker images -q -f dangling=true | xargs docker rmi -f
	for i in fnproject/fn-test-utils fnproject/fn-status-checker fnproject/hello fnproject/dind fnproject/fnserver ; do \
	    docker images "$$i" --format '{{ .ID }}\t{{ .Repository }}\t{{ .Tag}}' | while read id repo tag; do \
	        if [ "$$tag" = "<none>" ]; then docker rmi "$$id"; else docker rmi "$$repo:$$tag"; fi; done; done

.PHONY: release-fnserver
release-fnserver:
	./release.sh

.PHONY: build-dind
build-dind:
	(cd images/dind && ./build.sh)

.PHONY: release-dind
release-dind:
	(cd images/dind && ./release.sh)

.PHONY: fn-status-checker
fn-status-checker: checkfmt
	cd images/fn-status-checker && ./build.sh

.PHONY: fn-test-utils
fn-test-utils: checkfmt
	cd images/fn-test-utils && ./build.sh

.PHONY: test-middleware
test-middleware: test-basic
	cd examples/middleware && go build

.PHONY: test-extensions
test-extensions: test-basic
	cd examples/extensions && go build

.PHONY: test-basic
test-basic: checkfmt pull-images fn-test-utils fn-status-checker
	./test.sh

.PHONY: test
test: checkfmt pull-images test-basic test-middleware test-extensions test-system

.PHONY: test-system
test-system: test-basic
	./system_test.sh sqlite3
	./system_test.sh mysql
	./system_test.sh postgres

.PHONY: img-busybox
img-busybox:
	docker pull busybox

.PHONY: img-hello
img-hello:
	docker pull fnproject/hello

.PHONY: img-mysql
img-mysql:
	/bin/bash -c "source ./helpers.sh && docker_pull_mysql"

.PHONY: img-postgres
img-postgres:
	/bin/bash -c "source ./helpers.sh && docker_pull_postgres"

.PHONY: img-minio
img-minio:
	/bin/bash -c "source ./helpers.sh && docker_pull_minio"

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


protos: api/agent/grpc/runner.pb.go
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

.PHONY: docker-test
docker-test:
	docker run -ti --privileged --rm -e FN_LOG_LEVEL=debug \
	-v /var/run/docker.sock:/var/run/docker.sock \
	-v ${CURDIR}:/go/src/github.com/fnproject/fn \
	-w /go/src/github.com/fnproject/fn \
	fnproject/go:dev go test \
	-v $(shell docker run --rm -ti -v ${CURDIR}:/go/src/github.com/fnproject/fn -w /go/src/github.com/fnproject/fn -e GOPATH=/go golang:alpine sh -c 'go list ./... | \
                                                                                                                                                          grep -v vendor | \
                                                                                                                                                          grep -v examples | \
                                                                                                                                                          grep -v test/fn-api-tests | \
                                                                                                                                                          grep -v test/fn-system-tests | \
                                                                                                                                                          grep -v images/fn-test-utils')

.PHONY: all
all: dep generate build
