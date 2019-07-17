# build stage
FROM golang:1.12.7-alpine3.10 AS build-env
ENV D=/go/src/github.com/fnproject/fn/images/fn-status-checker
ADD . $D
RUN cd $D && go build -ldflags="-s -w" -o fn-status-checker && cp fn-status-checker /tmp/

# final stage
FROM alpine:3.10
WORKDIR /function
COPY --from=build-env /tmp/fn-status-checker /function
ENTRYPOINT ["./fn-status-checker"]
