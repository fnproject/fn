# build stage
FROM golang:1.10-alpine AS build-env
RUN apk --no-cache add git
ENV D=/go/src/github.com/fnproject/fn/images/fn-test-utils
RUN go get -u github.com/golang/dep/cmd/dep
ADD Gopkg.* $D/
RUN cd $D && dep ensure --vendor-only
ADD . $D
RUN cd $D && go build -o fn-test-utils && cp fn-test-utils /tmp/

# final stage
FROM alpine
WORKDIR /function
COPY --from=build-env /tmp/fn-test-utils /function
ENTRYPOINT ["./fn-test-utils"]
