set -e
docker tag fnproject/fn-test-utils:latest $DOCKER_USER/fn-test-utils:latest
docker push $DOCKER_USER/fn-test-utils:latest
