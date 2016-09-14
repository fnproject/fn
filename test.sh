export GO15VENDOREXPERIMENT=1

docker run --rm -it -v /var/run/docker.sock:/var/run/docker.sock \
    -e IGNORE_MEMORY=1 \
    -e LOG_LEVEL=debug \
    -e GOPATH="$PWD/../../../.." \
    -v "$PWD":"$PWD" -w "$PWD" iron/go-dind go test -v $(go list ./... | grep -v /vendor/ | grep -v /examples/)