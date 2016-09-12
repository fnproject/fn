export GO15VENDOREXPERIMENT=1
export IGNORE_MEMORY=1

docker run --rm -it -v /var/run/docker.sock:/var/run/docker.sock \
    -e TEST_DOCKER_USERNAME \
    -e TEST_DOCKER_PASSWORD \
    -e CI="$CI" \
    -e LOG_LEVEL=debug \
    -e GOPATH="$PWD/../../../.." \
    -v "$PWD":"$PWD" -w "$PWD" iron/go-dind go test -v $(go list ./... | grep -v /vendor/)