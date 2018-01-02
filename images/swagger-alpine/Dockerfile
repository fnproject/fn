FROM amd64/alpine:3.5

RUN apk add --no-cache curl
RUN curl -L https://github.com/go-swagger/go-swagger/releases/download/0.13.0/swagger_linux_amd64 -o swagger
RUN chmod +x swagger
ENTRYPOINT ["./swagger", "validate", "/go/src/github.com/fnproject/fn/docs/swagger.yml"]
