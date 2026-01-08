# build stage
FROM golang:1.10-alpine AS build-env
RUN apk --no-cache add build-base git bzr mercurial gcc
ENV D=/go/src/github.com/fnproject/fn
ADD . $D
RUN cd $D/cmd/fnserver && go build -o fn-alpine && cp fn-alpine /tmp/

# final stage: copy fn server binary from build-env to runtime image
FROM alpine:latest
WORKDIR /app
COPY --from=build-env /tmp/fn-alpine /app/fnserver
ENV FN_NODE_TYPE=api
CMD ["./fnserver"]
