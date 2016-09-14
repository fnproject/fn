FROM iron/go:dev

ADD . $GOPATH/src/github.com/iron-io/functions/examples/twitter
WORKDIR $GOPATH/src/github.com/iron-io/functions/examples/twitter

RUN go get .

ENTRYPOINT ["go", "run", "main.go"]