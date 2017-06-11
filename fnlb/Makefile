# Just builds
.PHONY: all build

build:
	go build

docker-build:
	docker pull funcy/go:dev
	docker run --rm -v ${GOPATH}/src/gitlab-odx.oracle.com/odx/functions:/go/src/gitlab-odx.oracle.com/odx/functions \
		-w /go/src/gitlab-odx.oracle.com/odx/functions/fnlb funcy/go:dev go build -o fnlb-alpine
	docker build --build-arg HTTP_PROXY -t funcy/fnlb:latest .

docker-run: docker-build
	docker run --rm -it --net=host -p 8081:8081 funcy/fnlb

all: build
