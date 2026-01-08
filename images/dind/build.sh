#!/bin/bash
set -exo pipefail

DIND_VERSION=24.0.9-dind

# Match version with Docker version
docker_info=$(docker run --rm docker:${DIND_VERSION} docker -v 2>/dev/null | grep "^Docker version")
version=$(echo $docker_info | cut -d ' ' -f 3 | tr -d ,)

echo "Version: $version"

M=$(echo $version | cut -d '.' -f 1)
Mm=$(echo $version | cut -d '.' -f 1,2)

# MultArch build start
export BUILDX_PLATFORMS="${BUILDX_PLATFORMS:-linux/amd64,linux/arm64}"

docker buildx create --name fnmultiarchbuilder --use
# Tag these up so that they're available for the local build process,
# if necessary
docker buildx build --build-arg HTTPS_PROXY --build-arg HTTP_PROXY --build-arg DIND_VERSION=${DIND_VERSION} --platform ${BUILDX_PLATFORMS} \
-t fnproject/dind:latest -t fnproject/dind:$version -t fnproject/dind:$Mm -t fnproject/dind:$M .

# load the single arch image locally (TODO: check later on whether we need it....)
docker buildx build --load --build-arg DIND_VERSION=${DIND_VERSION} -t fnproject/dind:latest .

docker images
