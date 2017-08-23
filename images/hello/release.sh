set -e

docker build -t fnproject/hello:latest .

docker push fnproject/hello:latest
