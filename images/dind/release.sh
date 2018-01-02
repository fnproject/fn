set -e

./build.sh

# Match version with Docker version
version=$(docker run --rm -v "$PWD":/app treeder/bump  --extract --input "`docker -v`")
echo "Version: $version"

docker tag fnproject/dind:latest fnproject/dind:$version

docker push fnproject/dind:latest
docker push fnproject/dind:$version
