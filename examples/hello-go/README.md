set -ex 

docker run --rm -v "$PWD":/go/src/github.com/treeder/hello -w /go/src/github.com/treeder/hello iron/go:dev go build -o hello
docker build -t iron/hello .
