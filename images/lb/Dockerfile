# build stage
FROM golang:1.14-alpine AS build-env
RUN apk --no-cache add build-base git bzr mercurial gcc
ENV D=/go/src/github.com/fnproject/fn
ADD . $D
RUN cd $D/cmd/fnserver && go build -o fn-alpine && cp fn-alpine /tmp/

# final stage: the local fnproject/dind:latest will be either built afresh or
# whatever is the latest from master, depending on whether we're releasing
# a newer cut.
FROM alpine:latest
WORKDIR /app
COPY --from=build-env /tmp/fn-alpine /app/fnserver
ENV FN_NODE_TYPE=lb
CMD ["./fnserver"]
