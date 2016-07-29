FROM golang:1.6.2

ADD . $GOPATH/src/func
WORKDIR $GOPATH/src/func

RUN go get .

ENTRYPOINT ["go", "run", "main.go"]