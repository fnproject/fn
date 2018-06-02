#!/bin/bash
set -exuo pipefail

user="fnproject"
image="fnserver"

# ensure working dir is clean
git status
if [[ -z $(git status -s) ]]
then
  echo "tree is clean"
else
  echo "tree is dirty, please commit changes before running this"
  exit 1
fi

#
# check if fn tag is already on docker repo
#
RELEASE_BRANCH=origin/master
FN_TAG="$(git tag --merged "$RELEASE_BRANCH" --sort='v:refname' '[0-9]*' | tail -1)"
[[ -z "$FN_TAG" ]] && FN_TAG="0.0.0"

set +e
docker pull $user/$image:$FN_TAG
IMG_STATUS=$?
set -e

if [ $IMG_STATUS -eq 0 ]; then
    echo "$user/$image:$FN_TAG already exists in docker repo"
    exit 0
fi

# Finally, push docker images
docker tag $user/$image:latest $user/$image:$version
docker push $user/$image:$version
docker push $user/$image:latest

