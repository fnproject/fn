set -ex

./build.sh

docker push funcy/lambda:node-4
