#!/bin/bash
set -exuo pipefail

user="fnproject"
image="fnserver"
image_deprecated="functions"

# ensure working dir is clean
git status
if [[ -z $(git status -s) ]]
then
  echo "tree is clean"
else
  echo "tree is dirty, please commit changes before running this"
  exit 1
fi

version=$(grep -m1 -Eo "[0-9]+\.[0-9]+\.[0-9]+" $version_file)
echo "Version: $version"

# Push the version bump and tags laid down previously
gtag=$image-$version
git push
git push origin $version

# Finally, push docker images
docker push $user/$image:$version
docker push $user/$image:latest

# Deprecated images, should remove this sometime in near future
docker tag $user/$image:latest $user/$image_deprecated:$version
docker tag $user/$image:latest $user/$image_deprecated:latest
docker push $user/$image_deprecated:$version
docker push $user/$image_deprecated:latest

# release test utils docker image
(cd images/fn-test-utils && ./release.sh)
