set -ex

# ./build.sh

# Match version with Docker version
version=$(docker run --rm -v "$PWD":/app treeder/bump  --extract --input "`docker -v`")
echo "Version: $version"
M=$(docker run --rm treeder/bump --format M --input "$version")
Mm=$(docker run --rm treeder/bump --format M.m --input "$version")

docker tag fnproject/dind:latest fnproject/dind:$version
# be nice to have bump image do all of this tagging and pushing too (mount docker sock and do it all)
docker tag fnproject/dind:$version fnproject/dind:$Mm
docker tag fnproject/dind:$version fnproject/dind:$M
 
docker push fnproject/dind:$version
docker push fnproject/dind:$Mm
docker push fnproject/dind:$M
docker push fnproject/dind:latest
