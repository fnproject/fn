FROM alpine

RUN apk --update upgrade && \
    apk add curl ca-certificates && \
        update-ca-certificates && \
            rm -rf /var/cache/apk/*

WORKDIR /app
ADD fnlb-alpine /app/fnlb
ENTRYPOINT ["./fnlb"]
