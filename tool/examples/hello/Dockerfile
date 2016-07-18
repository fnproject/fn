FROM iron/go:dev

ADD . /go/src/github.com/treeder/hello
WORKDIR /go/src/github.com/treeder/hello
RUN go build github.com/treeder/hello

ENTRYPOINT ["/go/src/github.com/treeder/hello/hello"]
