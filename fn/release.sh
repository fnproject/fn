#!/bin/bash

set -ex

# cd lambda
# ./release.sh
# cd ..

# make dep
# make release

version=$1

# For GitHub
# url='https://api.github.com/repos/treeder/functions/releases'
# output=$(curl -s -u $GH_DEPLOY_USER:$GH_DEPLOY_KEY -d "{\"tag_name\": \"$version\", \"name\": \"$version\"}" $url)
# upload_url=$(echo "$output" | python -c 'import json,sys;obj=json.load(sys.stdin);print obj["upload_url"]' | sed -E "s/\{.*//")
# html_url=$(echo "$output" | python -c 'import json,sys;obj=json.load(sys.stdin);print obj["html_url"]')
# curl --data-binary "@fn_linux"  -H "Content-Type: application/octet-stream" -u $GH_DEPLOY_USER:$GH_DEPLOY_KEY $upload_url\?name\=fn_linux >/dev/null
# curl --data-binary "@fn_mac"    -H "Content-Type: application/octet-stream" -u $GH_DEPLOY_USER:$GH_DEPLOY_KEY $upload_url\?name\=fn_mac >/dev/null
# curl --data-binary "@fn.exe"    -H "Content-Type: application/octet-stream" -u $GH_DEPLOY_USER:$GH_DEPLOY_KEY $upload_url\?name\=fn.exe >/dev/null

# For GitLab
# 1) Upload files: https://docs.gitlab.com/ee/api/projects.html#upload-a-file
upload_url='https://gitlab.oracledx.com/api/v3/projects/9/uploads'
curl --request POST --form "file=@fn_linux" --header "PRIVATE-TOKEN: $GITLAB_TOKEN" $upload_url
# curl --data-binary "@fn_mac"    -H "Content-Type: application/octet-stream" -u $GH_DEPLOY_USER:$GH_DEPLOY_KEY $upload_url\?name\=fn_mac >/dev/null
# curl --data-binary "@fn.exe"    -H "Content-Type: application/octet-stream" -u $GH_DEPLOY_USER:$GH_DEPLOY_KEY $upload_url\?name\=fn.exe >/dev/null

# url='https://gitlab.oracledx.com/api/v3/projects/9/uploads'
# output=$(curl -s -u $GH_DEPLOY_USER:$GH_DEPLOY_KEY -d "{\"tag_name\": \"$version\", \"name\": \"$version\"}" $url)


# upload_url=$(echo "$output" | python -c 'import json,sys;obj=json.load(sys.stdin);print obj["upload_url"]' | sed -E "s/\{.*//")
# html_url=$(echo "$output" | python -c 'import json,sys;obj=json.load(sys.stdin);print obj["html_url"]')


# curl --data-binary "@fn_linux"  -H "Content-Type: application/octet-stream" -u $GH_DEPLOY_USER:$GH_DEPLOY_KEY $upload_url\?name\=fn_linux >/dev/null
# curl --data-binary "@fn_mac"    -H "Content-Type: application/octet-stream" -u $GH_DEPLOY_USER:$GH_DEPLOY_KEY $upload_url\?name\=fn_mac >/dev/null
# curl --data-binary "@fn.exe"    -H "Content-Type: application/octet-stream" -u $GH_DEPLOY_USER:$GH_DEPLOY_KEY $upload_url\?name\=fn.exe >/dev/null
 