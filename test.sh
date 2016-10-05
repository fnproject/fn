export GO15VENDOREXPERIMENT=1
export LOG_LEVEL=debug
export IGNORE_MEMORY=1

go test -v $(go list ./... | grep -v /vendor/ | grep -v /examples/) 