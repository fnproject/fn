set -ex

./build.sh

docker run --rm --privileged -it -e LOG_LEVEL=debug -e "DB=bolt:///app/data/bolt.db" -v $PWD/data:/app/data -p 8080:8080 iron/functions
