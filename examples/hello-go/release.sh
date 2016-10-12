set -ex

USERNAME=iron
IMAGE=hello

# build it
./build.sh
# test it
echo '{"name":"Johnny"}' | docker run --rm -i $USERNAME/hello
# tag it
docker run --rm -v "$PWD":/app treeder/bump patch
version=`cat VERSION`
echo "version: $version"
docker tag $USERNAME/$IMAGE:latest $USERNAME/$IMAGE:$version
# push it
docker push $USERNAME/$IMAGE:latest
docker push $USERNAME/$IMAGE:$version
