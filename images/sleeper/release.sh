set -e

docker build -t fnproject/sleeper:latest .

docker push fnproject/sleeper:latest
