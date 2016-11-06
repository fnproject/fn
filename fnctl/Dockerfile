FROM alpine


RUN apk --update upgrade && \
    apk add curl ca-certificates && \
    update-ca-certificates && \
    rm -rf /var/cache/apk/*

COPY entrypoint.sh /
COPY fnctl /
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]