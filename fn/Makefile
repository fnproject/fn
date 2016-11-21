all: vendor
	go build -o fn
	./fn

docker: vendor
	GOOS=linux go build -o fn
	docker build -t iron/fn .
	docker push iron/fn

vendor:
	glide install -v

test:
	go test -v $(shell glide nv)

release: docker
	GOOS=linux go build -o fn_linux
	GOOS=darwin go build -o fn_mac
	GOOS=windows go build -o fn.exe
