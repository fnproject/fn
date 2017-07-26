all: vendor build	
	./fn

build: 
	go build -o fn

docker: vendor
	GOOS=linux go build -o fn
	docker build -t treeder/fn .
	docker push treeder/fn

dep:
	glide install -v

dep-up:
	glide up -v

test:
	go test $(go list ./... | grep -v /vendor/ | grep -v /tests)

test-integration:
	cd tests/ && go test -v ./...; cd ..

release:
	GOOS=linux go build -o fn_linux
	GOOS=darwin go build -o fn_mac
	GOOS=windows go build -o fn.exe
	docker run --rm -v ${PWD}:/go/src/github.com/fnproject/fn/cli -w /go/src/github.com/fnproject/fn/cli funcy/go:dev go build -o fn_alpine
