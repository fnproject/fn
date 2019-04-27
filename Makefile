# Just builds
.PHONY: mod
dep:
	GO111MODULE=on GOFLAGS=-mod=vendor go mod vendor -v

.PHONY: mod-up
dep-up:
	go get -u

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
	for i in fnproject/fn-test-utils fnproject/fn-status-checker fnproject/dind fnproject/fnserver fnproject/fn-test-volume ; do \
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

.PHONY: fn-test-volume
fn-test-volume:
	cd images/fn-test-volume && ./build.sh

.PHONY: test-middleware
test-middleware: test-basic
	cd examples/middleware && go build

.PHONY: test-extensions
test-extensions: test-basic
	cd examples/extensions && go build

.PHONY: test-basic
test-basic: checkfmt pull-images fn-test-utils fn-status-checker fn-test-volume
	./test.sh

.PHONY: test
test: checkfmt pull-images test-basic test-middleware test-extensions test-system

.PHONY: test-system
test-system:
	./system_test.sh sqlite3 $(run)
	./system_test.sh mysql $(run)
	./system_test.sh postgres $(run)

.PHONY: img-busybox
img-busybox:
	docker pull busybox

.PHONY: img-mysql
img-mysql:
	/bin/bash -c "source ./helpers.sh && docker_pull_mysql"

.PHONY: img-postgres
img-postgres:
	/bin/bash -c "source ./helpers.sh && docker_pull_postgres"

.PHONY: pull-images
pull-images: img-mysql img-postgres img-busybox

.PHONY: test-datastore
test-datastore:
	cd api/datastore && go test  ./...

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

.PHONY: docker-test
docker-test:
	docker run -ti --privileged --rm -e FN_LOG_LEVEL=debug \
	-v /var/run/docker.sock:/var/run/docker.sock \
	-v ${CURDIR}:/go/src/github.com/fnproject/fn \
	-w /go/src/github.com/fnproject/fn \
	fnproject/go:dev go test \
	-v $(shell docker run --rm -ti -v ${CURDIR}:/go/src/github.com/fnproject/fn -w /go/src/github.com/fnproject/fn -e GOFLAGS -e GO111MODULE -e GOPATH=/go golang:alpine sh -c 'go list ./... | \
                                                                                                                                                          grep -v vendor | \
                                                                                                                                                          grep -v examples | \
                                                                                                                                                          grep -v test/fn-api-tests | \
                                                                                                                                                          grep -v test/fn-system-tests | \
                                                                                                                                                          grep -v images/fn-test-utils')

.PHONY: all
all: mod generate build
