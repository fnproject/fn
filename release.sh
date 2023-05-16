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

version_file="api/version/version.go"
if [ -z $(grep -m1 -Eo "[0-9]+\.[0-9]+\.[0-9]+" $version_file) ]; then
  echo "did not find semantic version in $version_file"
  exit 1
fi
perl -i -pe 's/\d+\.\d+\.\K(\d+)/$1+1/e' $version_file
version=$(grep -m1 -Eo "[0-9]+\.[0-9]+\.[0-9]+" $version_file)
echo "Version: $version"

# Push the version bump and tags laid down previously
git add -u
git commit -m "$image: v$version release [skip ci]"
git push  origin "sunnseth/test-push"

# Finally, push docker images
#docker tag $user/$image:latest $user/$image:$version
#docker push $user/$image

#(cd images/fn-test-utils && ./release.sh)
#(cd images/fn-status-checker && ./release.sh)

