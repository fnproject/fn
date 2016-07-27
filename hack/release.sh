#!/bin/bash
set -ex

user="iron"
service="functions"
version_file="api/version.go"
tag="latest"

if [ -z $(grep -m1 -Eo "[0-9]+\.[0-9]+\.[0-9]+" $version_file) ]; then
  echo "did not find semantic version in $version_file"
  exit 1
fi

perl -i -pe 's/\d+\.\d+\.\K(\d+)/$1+1/e' $version_file
version=$(grep -m1 -Eo "[0-9]+\.[0-9]+\.[0-9]+" $version_file)
echo "Version: $version"

docker run --rm -v "$PWD":/go/src/github.com/iron-io/functions -w /go/src/github.com/iron-io/functions iron/go:dev sh -c 'go build -o functions'
docker build -t iron/functions:latest .

git add -u
git commit -m "$service: $version release"
git tag -a "$version" -m "version $version"
git push
git push --tags

# Finally tag and push docker images
docker tag $user/$service:$tag $user/$service:$version

docker push $user/$service:$version
docker push $user/$service:$tag
