#!/bin/bash

set -ex

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

version_file="version.go"
if [ -z $(grep -m1 -Po '(?<= = ")\d+\.\d+\.\d+' $version_file) ]; then
  echo "did not find semantic version in $version_file"
  exit 1
fi
perl -i -pe 's/(?<= = ")\d+\.\d+\.\K(\d+)/$1+1/e' $version_file
version=$(grep -m1 -Po '(?<= = ")\d+\.\d+\.\d+' $version_file)
echo "Version: $version"

#cd lambda
#./release.sh
#cd ..

# make dep
make release

tag="fn-$version"
git add -u
git commit -m "fn tool: $version release [skip ci]"
# todo: might make sense to move this into it's own repo so it can have it's own versioning at some point
git tag -f -a $tag -m "fn version $version"
git push
git push origin $tag

# For GitHub
url='https://api.github.com/repos/fnproject/cli/releases'
output=$(curl -s -u $GH_DEPLOY_USER:$GH_DEPLOY_KEY -d "{\"tag_name\": \"$version\", \"name\": \"$version\"}" $url)
upload_url=$(echo "$output" | python -c 'import json,sys;obj=json.load(sys.stdin);print obj["upload_url"]' | sed -E "s/\{.*//")
html_url=$(echo "$output" | python -c 'import json,sys;obj=json.load(sys.stdin);print obj["html_url"]')
curl --data-binary "@fn_linux"  -H "Content-Type: application/octet-stream" -u $GH_DEPLOY_USER:$GH_DEPLOY_KEY $upload_url\?name\=fn_linux >/dev/null
curl --data-binary "@fn_mac"    -H "Content-Type: application/octet-stream" -u $GH_DEPLOY_USER:$GH_DEPLOY_KEY $upload_url\?name\=fn_mac >/dev/null
curl --data-binary "@fn.exe"    -H "Content-Type: application/octet-stream" -u $GH_DEPLOY_USER:$GH_DEPLOY_KEY $upload_url\?name\=fn.exe >/dev/null
curl --data-binary "@fn_alpine" -H "Content-Type: application/octet-stream" -u $GH_DEPLOY_USER:$GH_DEPLOY_KEY $upload_url\?name\=fn_alpine >/dev/null

# TODO: Add the download URLS to install.sh. Maybe we should make a template to generate install.sh
# TODO: Download URL's are in the output vars above under "url". Eg: "url":"/uploads/9a1848c5ebf2b83f8b055ac0e50e5232/fn.exe"
# sed "s/release=.*/release=\"$version\"/g" install.sh > install.sh.tmp
# mv install.sh.tmp install.sh
# TODO: then git commit and push?  Would be nice to do that along with the vrsion git push above
