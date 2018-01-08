set -ex

docker build --build-arg HTTPS_PROXY --build-arg HTTP_PROXY -t fnproject/dind:latest .
