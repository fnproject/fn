#!/bin/bash

travis=$(cat .travis.yml | grep '^go:' | sed 's/^go: \([0-9]*\.[0-9]*\)\(\.[0-9]*\)\{0,1\}$/\1/')
godep=$(cat Godeps/Godeps.json | grep 'GoVersion' | sed 's/.*"go\(.*\)".*$/\1/')
diff <(echo $travis) <(echo $godep)
