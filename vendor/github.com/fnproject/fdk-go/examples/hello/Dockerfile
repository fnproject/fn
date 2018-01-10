FROM fnproject/go:dev as build-stage
WORKDIR /function
ADD . /src
RUN go get github.com/fnproject/fdk-go
RUN cd /src && go build -o func
FROM fnproject/go
WORKDIR /function
COPY --from=build-stage /src/func /function/
ENTRYPOINT ["./func"]
