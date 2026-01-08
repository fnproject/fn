set -e
DOCKER_BUILDKIT=1 docker buildx build --platform ${BUILDX_PLATFORMS} --push -t fnproject/fn-status-checker:$1 .
