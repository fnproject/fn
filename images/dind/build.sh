set -ex

docker build --build-arg HTTP_PROXY -t funcy/dind:latest .
