set -ex 

USERNAME=iron
IMAGE=hello

# build it
docker run --rm -v "$PWD":/go/src/github.com/iron/hello -w /go/src/github.com/iron/hello iron/go:dev go build -o hello
docker build -t $USERNAME/$IMAGE .
