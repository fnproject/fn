# Just builds
dep:
	@ glide install
	rm -rf vendor/github.com/heroku/docker-registry-client/vendor

build:
	@ go build -o functions

build-docker:
	sh scripts/build-docker.sh

release:
	sh scripts/release.sh

test:
	sh scripts/test.sh

run-docker:
	sh scripts/run-docker.sh

run-simple:
	./functions

all: dep build