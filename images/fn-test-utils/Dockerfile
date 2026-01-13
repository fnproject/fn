# build stage
FROM golang:1.24-alpine AS build-env
ENV D=/go/src/github.com/fnproject/fn/images/fn-test-utils
ADD . $D
RUN cd $D && go build -o fn-test-utils && cp fn-test-utils /tmp/

# final stage
FROM alpine
RUN addgroup -g 1000 -S fn && adduser -S -u 1000 -G fn fn
WORKDIR /function
COPY --from=build-env /tmp/fn-test-utils /function
ENTRYPOINT ["./fn-test-utils"]
