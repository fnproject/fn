FROM docker:24.0.9-dind

RUN apk add --no-cache ca-certificates

COPY preentry.sh /usr/local/bin/

ENTRYPOINT ["preentry.sh"]

# USAGE: Add a CMD to your own Dockerfile to use this (NOT an ENTRYPOINT), eg:
# CMD ["./runner"]
