set -e

fn bump
fn build
fn push

docker build -t fnproject/error:latest .

docker push fnproject/error:latest
