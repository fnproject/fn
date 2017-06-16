#!/bin/sh

git stash -q --keep-index
trap "git stash pop -q" EXIT
make precommit
