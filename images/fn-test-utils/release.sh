set -e

docker build -t fnproject/fn-test-utils:latest .

docker push fnproject/fn-test-utils:latest
