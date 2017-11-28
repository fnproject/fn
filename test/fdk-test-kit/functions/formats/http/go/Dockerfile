FROM fnproject/go:dev as build-stage
WORKDIR /function
ADD . /src
RUN go get -u github.com/fnproject/fdk-go
WORKDIR /src
RUN go build -o func
FROM fnproject/go
WORKDIR /function
COPY --from=build-stage /src/func /function/
ENTRYPOINT ["./func"]
