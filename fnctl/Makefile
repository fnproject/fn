all: vendor
	go build -o fnctl
	./fnctl

docker: vendor
	GOOS=linux go build -o fnctl
	docker build -t iron/fnctl .
	docker push iron/fnctl

vendor:
	go get -u .
