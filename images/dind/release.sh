set -e

./build.sh

docker run --rm -v "$PWD":/app treeder/bump patch
version=`cat VERSION`
echo "version $version"

docker tag funcy/dind:latest funcy/dind:$version

docker push funcy/dind:latest
docker push funcy/dind:$version
