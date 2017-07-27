# Just builds
.PHONY: all build

build:
	go build

docker-build:
	docker pull funcy/go:dev
	docker run --rm -v ${GOPATH}/src/github.com/fnproject/fn:/go/src/github.com/fnproject/fn \
		-w /go/src/github.com/fnproject/fn/fnlb funcy/go:dev go build -o fnlb-alpine
	docker build --build-arg HTTP_PROXY -t funcy/fnlb:latest .

all: build
