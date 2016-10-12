USERNAME=iron
# build it
docker build -t $USERNAME/hello:ruby .
# test it
echo '{"name":"Johnny"}' | docker run --rm -i $USERNAME/hello
# tag it
docker run --rm -v "$PWD":/app treeder/bump patch
# push it
docker push $USERNAME/hello:ruby
