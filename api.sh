set -ex

./build.sh

docker run --rm --privileged -it -p 8080:8080 -e LOG_LEVEL=debug -v $PWD/bolt.db:/app/bolt.db iron/functions