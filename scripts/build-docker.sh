set -ex

docker run --rm -v "$PWD":/go/src/github.com/iron-io/functions -w /go/src/github.com/iron-io/functions iron/go:dev go build -o functions-alpine
docker build -t iron/functions:latest .
