set -e

./build.sh
docker push fnproject/fn-test-utils:latest
