# build stage
FROM golang:1.10-alpine AS build-env
RUN apk --no-cache add build-base git bzr mercurial gcc
ENV D=/go/src/github.com/fnproject/fn
ADD . $D
RUN cd $D/cmd/fnserver && go build -o fn-alpine && cp fn-alpine /tmp/

# final stage: using docker:dind as base image
ARG DIND_VERSION
ARG base_image=docker:${DIND_VERSION}
FROM ${base_image}

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=build-env /tmp/fn-alpine /app/fnserver
ENV FN_NODE_TYPE=pure-runner
CMD ["./fnserver"]
