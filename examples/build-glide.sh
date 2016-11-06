EXAMPLE=`pwd`

cd ..
docker build -f Dockerfile.glide -t glide .
cd $EXAMPLE