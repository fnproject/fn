set -e
# single arch image that used in CI pipeline when system test is run
docker build --build-arg HTTPS_PROXY --build-arg HTTP_PROXY -t fnproject/fn-test-volume:latest .
