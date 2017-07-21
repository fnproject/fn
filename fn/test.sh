set -ex

make build
export fn="$(pwd)/fn"
$fn --version

# This tests all the quickstart commands on the cli on a live server
rm -rf tmp
mkdir tmp
cd tmp
$fn init --runtime go $DOCKER_USER/fn-test-go
$fn test

docker rm --force functions || true # just in case
docker run --name functions -d -v /var/run/docker.sock:/var/run/docker.sock -p 8080:8080 funcy/functions

sleep 10
$fn apps l
$fn apps create myapp
$fn deploy myapp
$fn call myapp /fn-test-go3

docker rm --force functions

cd ..
