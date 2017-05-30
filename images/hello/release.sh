set -e

docker build -t funcy/hello:latest .

docker push funcy/hello:latest
