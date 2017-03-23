#!/bin/bash
set -ex

user="iron"
service="functions"
version_file="api/version/version.go"
tag="latest"

if [ -z $(grep -m1 -Eo "[0-9]+\.[0-9]+\.[0-9]+" $version_file) ]; then
  echo "did not find semantic version in $version_file"
  exit 1
fi

perl -i -pe 's/\d+\.\d+\.\K(\d+)/$1+1/e' $version_file
version=$(grep -m1 -Eo "[0-9]+\.[0-9]+\.[0-9]+" $version_file)
echo "Version: $version"

make docker-build

sed "s/release=.*/release=\"$version\"/g" fn/install.sh > fn/install.sh.tmp
mv fn/install.sh.tmp fn/install.sh

git add -u
git commit -m "$service: $version release [skip ci]"
git tag -f -a "$version" -m "version $version"
git push
git push origin $version

# Finally tag and push docker images
docker tag $user/$service:$tag $user/$service:$version

docker push $user/$service:$version
docker push $user/$service:$tag

cd fn
./release.sh $version
cd ..
