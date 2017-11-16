#!/bin/bash

testlist=`find -L "$@" -type f -executable -name test.sh`

for test in $testlist
do
    cd $(dirname $test)
    echo "${test}"
    ./test.sh
    cd ..
done