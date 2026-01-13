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

RELEASE_TAG="latest"
if [ "$CIRCLE_BRANCH" != "master" ]; then
  # for branch that want to test the docker build/push step
  RELEASE_TAG="${CIRCLE_BRANCH/\//_}"
fi

# Build and push multi arch image
DOCKER_BUILDKIT=1
docker buildx create --name fnmultiarchbuilder --use
docker buildx build --platform ${BUILDX_PLATFORMS} \
	--push -t $user/$image:$RELEASE_TAG -t $user/$image:${version} .

(cd images/fn-test-utils && ./release.sh $RELEASE_TAG)
(cd images/fn-status-checker && ./release.sh $RELEASE_TAG)

# Push the version bump and tags laid down previously
if [ "$CIRCLE_BRANCH" = "master" ]; then
  git add -u
  git commit -m "$image: v$version release [skip ci]"
  git tag -f -a "v$version" -m "version v$version"
  git push --tags origin master
else
  echo "CIRCLE_BRANCH is not master. Skip tagging the repo."
fi


