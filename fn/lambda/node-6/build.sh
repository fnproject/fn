set -ex

docker build --build-arg HTTP_PROXY -t funcy/lambda:node-6 .
