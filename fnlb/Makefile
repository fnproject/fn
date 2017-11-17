# Just builds
.PHONY: all build

build:
	go build

docker-build:
	docker pull fnproject/go:dev
	docker run --rm -v ${GOPATH}/src/github.com/fnproject/fn:/go/src/github.com/fnproject/fn \
		-w /go/src/github.com/fnproject/fn/fnlb fnproject/go:dev go build -o fnlb-alpine
	docker build --build-arg HTTP_PROXY -t fnproject/fn-lb:latest .

all: build
