# Top level test script to start all other tests

set -ex

go test -v $(go list ./... | grep -v vendor | grep -v examples | grep -v tool | grep -v fn)
cd fn && make build && make test
# TODO: should we install fn here to use throughout?
FN="$(pwd)/fn"
cd ..

# TODO: Test a bunch of the examples using fn test when ready
# checker tests env vars
cd examples/checker
./test.sh
