# build stage
FROM golang:1.14-alpine AS build-env
RUN apk --no-cache add git
ENV D=/go/src/github.com/fnproject/fn/images/fn-status-checker
RUN go get -u github.com/golang/dep/cmd/dep
ADD Gopkg.* $D/
RUN cd $D && dep ensure --vendor-only
ADD . $D
RUN cd $D && go build -ldflags="-s -w" -o fn-status-checker && cp fn-status-checker /tmp/

# final stage
FROM alpine
WORKDIR /function
COPY --from=build-env /tmp/fn-status-checker /function
ENTRYPOINT ["./fn-status-checker"]
