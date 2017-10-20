FROM fnproject/dind
RUN apk add --no-cache ca-certificates build-base go git bzr mercurial gcc

ENV GOPATH=/go
RUN env
ENV PATH=$PATH:/go/bin
RUN env
ENV D=/go/src/github.com/fnproject/fn

ADD . $D
RUN cd $D && go build -o /app/functions

WORKDIR /app
CMD ["./functions"]

# For building extensions
ONBUILD ARG REPO
ONBUILD ENV REPOPATH=$GOPATH/src/$REPO
ONBUILD ADD . $REPOPATH
# dep just way too slow and error prone... :(
# It might be a good idea to move interfaces and models to a new repo with minimal depenencies, then this might work nice
# ONBUILD RUN go get -u github.com/golang/dep/cmd/dep
# ONBUILD RUN cd $REPOPATH && dep init && dep ensure
# Try doing regular go get:
ONBUILD RUN cd $REPOPATH && go get
# ONBUILD ADD main.go $D
ONBUILD RUN cd $REPOPATH && go build -o /app/functions
