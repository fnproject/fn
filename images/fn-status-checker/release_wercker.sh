set -e
docker tag fnproject/fn-status-checker:latest $DOCKER_USER/fn-status-checker:latest
docker push $DOCKER_USER/fn-status-checker:latest
