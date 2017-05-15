set -ex

./build.sh
docker push treeder/functions-lambda:nodejs4.3
