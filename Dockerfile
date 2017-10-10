# build stage
# FROM golang:alpine
FROM fnproject/dind
RUN apk add --no-cache ca-certificates build-base go git bzr mercurial gcc

ENV GOPATH=/go
ENV D=/go/src/github.com/fnproject/fn
# If dep ever gets decent enough to use, try `dep ensure --vendor-only` from here: https://medium.com/travis-on-docker/triple-stage-docker-builds-with-go-and-angular-1b7d2006cb88
#RUN go get -u github.com/Masterminds/glide
#ADD glide.* $D/
#RUN cd $D && glide install -v

ADD . $D
# RUN cd $D && go build -o fn-alpine && cp fn-alpine /tmp/
# TODO: ADD BACK-  RUN cd $D && go build -o /app/functions

# final stage
# FROM fnproject/dind
WORKDIR /app
# COPY --from=build-env /tmp/fn-alpine /app/functions
CMD ["./functions"]

# For building extensions
ONBUILD ARG REPO
ONBUILD RUN echo "YOOOO $DIR"
ONBUILD RUN echo $REPO
# ONBUILD ENV GOPATH=$GOPATH:$DIR
# ONBUILD RUN echo $GOPATH
ONBUILD ADD . $GOPATH/src/$REPO
ONBUILD ADD main.go $D
ONBUILD RUN cd $D && go build -o /app/functions
