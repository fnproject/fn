# build stage
FROM golang:1.24-alpine AS build-env
ENV D=/go/src/github.com/fnproject/fn
ADD . $D
RUN cd $D/cmd/fnserver && go build -o fn-alpine && cp fn-alpine /tmp/

# final stage: copy fn server binary from build-env to runtime image
FROM alpine:latest
WORKDIR /app
COPY --from=build-env /tmp/fn-alpine /app/fnserver
ENV FN_NODE_TYPE=lb
CMD ["./fnserver"]
