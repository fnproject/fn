#!/bin/bash
# For local development, build a single arch dind image if fnproject/dind is required by other image such as images/runner

set -exo pipefail

docker build --build-arg HTTPS_PROXY --build-arg HTTP_PROXY -t fnproject/dind:latest .
