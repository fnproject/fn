FROM iron/go:dev

ADD . $GOPATH/src/github.com/iron-io/functions/examples/blog
WORKDIR $GOPATH/src/github.com/iron-io/functions/examples/blog

RUN go get .

ENTRYPOINT ["go", "run", "main.go"]