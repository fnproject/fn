FROM alpine

RUN apk --update upgrade && \
    apk add --no-cache curl ca-certificates && \
    update-ca-certificates

WORKDIR /app
ADD fnlb-alpine /app/fnlb
ENTRYPOINT ["./fnlb"]
