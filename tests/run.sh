set -ex

./build.sh
docker run --rm --link worker-api iron/worker-test bundle exec ruby test.rb
