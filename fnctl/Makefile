all: vendor
	go build -o fnctl
	./fnctl

docker: vendor
	GOOS=linux go build -o fnctl
	docker build -t iron/fnctl .
	docker push iron/fnctl

vendor:
	glide install -v

test:
	go test -v $(shell glide nv)

release: docker
	GOOS=linux go build -o fnctl_linux
	GOOS=darwin go build -o fnctl_mac
	GOOS=windows go build -o fnctl.exe
