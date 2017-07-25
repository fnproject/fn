set -ex

make build
export fn="$(pwd)/fn"
$fn --version

# This tests all the quickstart commands on the cli on a live server
rm -rf tmp
mkdir tmp
cd tmp
funcname="tn-test-go"
$fn init --runtime go $DOCKER_USERNAME/$funcname
$fn test

someport=50080
docker rm --force functions || true # just in case
docker run --name functions -d -v /var/run/docker.sock:/var/run/docker.sock -p $someport:8080 funcy/functions

sleep 10
export API_URL="http://localhost:$someport"
$fn apps l
$fn apps create myapp
$fn deploy myapp
$fn call myapp $DOCKER_USERNAME/$funcname

docker rm --force functions

cd ..
