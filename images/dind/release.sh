set -e

./build.sh

docker run --rm -v "$PWD":/app treeder/bump patch
version=`cat VERSION`
echo "version $version"

docker tag treeder/dind:latest treeder/dind:$version

docker push treeder/dind:latest
docker push treeder/dind:$version
