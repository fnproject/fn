#!/bin/bash
# find and output all Go files that are not correctly formatted
set -euo pipefail

# Find all .go files except those under vendor/ or .git, run gofmt -l on them
OUT=$(find . ! \( -path ./vendor -prune \) ! \( -path ./.git -prune \) -name '*.go' -exec gofmt -l {} +)

if [ -n "$OUT" ]; then
  echo "gofmt reported formatting errors in:"
  echo "$OUT"
  exit 1
fi
