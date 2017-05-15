set -ex

docker build --build-arg HTTP_PROXY -t treeder/functions-lambda:nodejs4.3 .
