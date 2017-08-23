set -ex

make build
export fn="$(pwd)/fn"
$fn --version

go test $(go list ./... | grep -v /vendor/ | grep -v /tests)

# This tests all the quickstart commands on the cli on a live server
rm -rf tmp
mkdir tmp
cd tmp
funcname="fn-test-go"
$fn init --runtime go $DOCKER_USER/$funcname
$fn test

someport=50080
docker rm --force functions || true # just in case
docker run --name functions -d -v /var/run/docker.sock:/var/run/docker.sock -p $someport:8080 fnproject/functions
sleep 10
docker logs functions

export API_URL="http://localhost:$someport"
$fn apps l
$fn apps create myapp
$fn apps l
$fn deploy myapp
$fn call myapp $funcname

docker rm --force functions

cd ..
