#!/bin/bash
# find and output all Go files that are not correctly formatted
set -euo pipefail

# Find all .go files except those under vendor/ or .git, run gofmt -l on them
find . ! \( -path ./vendor -prune \) ! \( -path ./.git -prune \) -name '*.go' -exec gofmt -s -w {} +
