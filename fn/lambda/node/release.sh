set -ex

./build.sh
docker push iron/functions-lambda:nodejs4.3
