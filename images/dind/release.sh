#!/bin/bash

set -exo pipefail

# Ensure working dir is clean
git status
if [[ -z $(git status -s) ]]
then
  echo "tree is clean"
else
  echo "tree is dirty, please commit changes before running this"
  exit 1
fi

# This script should be run after its sibliing, build.sh, and
# after any related tests have passed.

# Match version with Docker version
docker_info=$(docker run --rm fnproject/dind:latest docker -v 2>/dev/null | grep "^Docker version")
version=$(echo $docker_info | cut -d ' ' -f 3 | tr -d ,)

echo "Version: $version"

M=$(echo $version | cut -d '.' -f 1)
Mm=$(echo $version | cut -d '.' -f 1,2)

# Calculate new release version
DIND_NEW=$(echo "$DIND_PREV" | perl -pe 's/\d+\.\d+\.\K(\d+)/$1+1/e')

# Add appropriate docker tags
docker tag fnproject/dind:latest fnproject/dind:$version
# be nice to have bump image do all of this tagging and pushing too (mount docker sock and do it all)
docker tag fnproject/dind:$version fnproject/dind:$Mm
docker tag fnproject/dind:$version fnproject/dind:$M
docker tag fnproject/dind:$version fnproject/dind:release-$DIND_NEW
 
docker push fnproject/dind:$version
docker push fnproject/dind:$Mm
docker push fnproject/dind:$M
docker push fnproject/dind:release-$DIND_NEW
docker push fnproject/dind:latest

# Mark this release with a tag
# No code changes so only the tag requires a push
git tag -f -a "$DIND_NEW" -m "DIND release $DIND_NEW of $version"
git push origin "$DIND_NEW"

