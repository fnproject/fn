#!/bin/bash
set -exo pipefail

docker build --build-arg HTTPS_PROXY --build-arg HTTP_PROXY -t fnproject/dind:latest .

# Match version with Docker version
docker_info=$(docker run --rm fnproject/dind:latest docker -v 2>/dev/null | grep "^Docker version")
version=$(echo $docker_info | cut -d ' ' -f 3 | tr -d ,)

echo "Version: $version"

M=$(echo $version | cut -d '.' -f 1)
Mm=$(echo $version | cut -d '.' -f 1,2)

# Tag these up so that they're available for the local build process,
# if necessary

docker tag fnproject/dind:latest fnproject/dind:$version
# be nice to have bump image do all of this tagging and pushing too (mount docker sock and do it all)
docker tag fnproject/dind:$version fnproject/dind:$Mm
docker tag fnproject/dind:$version fnproject/dind:$M
