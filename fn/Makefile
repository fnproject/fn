all: vendor build	
	./fn

build: 
	go build -o fn

docker: vendor
	GOOS=linux go build -o fn
	docker build -t iron/fn .
	docker push iron/fn

vendor:
	dep ensure --update

test:
	go test $(go list ./... | grep -v /vendor/)

release:
	GOOS=linux go build -o fn_linux
	GOOS=darwin go build -o fn_mac
	GOOS=windows go build -o fn.exe

# install locally
install: build
	sudo mv fn /usr/local/bin/