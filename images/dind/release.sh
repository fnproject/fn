set -e

./build.sh

docker run --rm -v "$PWD":/app treeder/bump patch
version=`cat VERSION`
echo "version $version"

docker tag fnproject/dind:latest fnproject/dind:$version

docker push fnproject/dind:latest
docker push fnproject/dind:$version
