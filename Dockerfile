# build stage
FROM golang:1.9-alpine AS build-env
RUN apk --no-cache add build-base git bzr mercurial gcc
ENV D=/go/src/github.com/fnproject/fn
RUN go get -u github.com/golang/dep/cmd/dep
ADD Gopkg.* $D/
RUN cd $D && dep ensure --vendor-only
ADD . $D
RUN cd $D && go build -o fn-alpine && cp fn-alpine /tmp/

# final stage
FROM fnproject/dind
WORKDIR /app
COPY --from=build-env /tmp/fn-alpine /app/fnserver
CMD ["./fnserver"]
