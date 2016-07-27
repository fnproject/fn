set -ex

docker run --rm -v "$PWD":/go/src/github.com/iron-io/functions -w /go/src/github.com/iron-io/functions iron/go:dev sh -c 'go build -o functions'
docker build -t iron/functions:latest .
docker run --rm -it -p 8080:8080 -e LOG_LEVEL=debug -v $PWD/bolt.db:/app/bolt.db iron/functions