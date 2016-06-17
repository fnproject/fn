set -ex

docker run --rm -v "$PWD":/go/src/github.com/iron-io/microgateway -w /go/src/github.com/iron-io/microgateway iron/go:dev sh -c 'go build -o gateway'
docker build -t iron/gateway:latest .
