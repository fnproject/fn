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

git pull

version_file="api/version/version.go"
if [ -z $(grep -m1 -Eo "[0-9]+\.[0-9]+\.[0-9]+" $version_file) ]; then
  echo "did not find semantic version in $version_file"
  exit 1
fi
perl -i -pe 's/\d+\.\d+\.\K(\d+)/$1+1/e' $version_file
version=$(grep -m1 -Eo "[0-9]+\.[0-9]+\.[0-9]+" $version_file)
echo "Version: $version"

make docker-build

git add -u
git commit -m "$image: $version release [skip ci]"
git tag -f -a "$version" -m "version $version"
git push
git push origin $version

# Finally tag and push docker images
docker tag $user/$image:latest $user/$image:$version
docker push $user/$image:$version
docker push $user/$image:latest

# Deprecated images, should remove this sometime in near future
docker tag $user/$image:latest $user/$image_deprecated:$version
docker tag $user/$image:latest $user/$image_deprecated:latest
docker push $user/$image_deprecated:$version
docker push $user/$image_deprecated:latest

# release test utils docker image
(cd images/fn-test-utils && ./release.sh)
