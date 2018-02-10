# build stage
FROM golang:1.9-alpine AS build-env
RUN apk --no-cache add git
ENV D=/go/src/github.com/fnproject/
RUN go get -u github.com/golang/dep/cmd/dep
ADD . $D
RUN cd $D && dep ensure --vendor-only
RUN cd $D && go build -o func && cp func /tmp/

# final stage
FROM alpine
WORKDIR /function
COPY --from=build-env /tmp/func /function
ENTRYPOINT ["./func"]
