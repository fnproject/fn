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

RELEASE_BRANCH=origin/master
DIND_TAG="$(git tag --merged "$RELEASE_BRANCH" --sort='v:refname' 'dind-*' | tail -1)"
[[ -z "$DIND_TAG" ]] && DIND_TAG="dind-0.0.0"

# Calculate new release version
DIND_NEW=$(echo "$DIND_TAG" | perl -pe 's/\d+\.\d+\.\K(\d+)/$1+1/e')

# Mark this release with a tag
# No code changes so only the tag requires a push
git tag -f -a "$DIND_NEW" -m "DIND release $DIND_NEW of $version"

