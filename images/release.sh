#!/bin/bash
set -ex

image_list="fn-test-utils"

# here add test images to be released as part of build process
for img in $image_list
do
    echo "Building image $img"
    (cd $img && ./release.sh)
done

