set -e

docker build -t fnproject/tester:latest .

docker push fnproject/tester:latest
