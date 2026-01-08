ARG DIND_VERSION=24.0.9-dind

# build stage
FROM golang:1.10-alpine AS build-env
RUN apk --no-cache add build-base git bzr mercurial gcc
ENV D=/go/src/github.com/fnproject/fn
ADD . $D
RUN cd $D/cmd/fnserver && go build -o fn-alpine && cp fn-alpine /tmp/

# final stage: using docker:dind as base image
FROM docker:${DIND_VERSION}

RUN apk add --no-cache ca-certificates

COPY ./images/dind/preentry.sh /usr/local/bin/

ENTRYPOINT ["preentry.sh"]

WORKDIR /app
COPY --from=build-env /tmp/fn-alpine /app/fnserver
CMD ["./fnserver"]
EXPOSE 8080
