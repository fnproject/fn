USERNAME=iron
# build it
docker build -t $USERNAME/hello .
# test it
echo '{"name":"Johnny"}' | docker run --rm -i $USERNAME/hello
# tag it
docker run --rm -v "$PWD":/app treeder/bump patch
docker tag $USERNAME/hello:latest $USERNAME/hello:`cat VERSION`
# push it
docker push $USERNAME/hello
