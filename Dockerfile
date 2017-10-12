FROM fnproject/dind
RUN apk add --no-cache ca-certificates build-base go git bzr mercurial gcc

ENV GOPATH=/go
ENV D=/go/src/github.com/fnproject/fn

ADD . $D
RUN cd $D && go build -o /app/functions

WORKDIR /app
CMD ["./functions"]

# For building extensions
ONBUILD ARG REPO
ONBUILD ADD . $GOPATH/src/$REPO
ONBUILD ADD main.go $D
ONBUILD RUN cd $D && go build -o /app/functions
