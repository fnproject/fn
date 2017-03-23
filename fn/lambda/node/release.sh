set -ex

./build.sh
docker push iron/functions-lambda:node4.3
