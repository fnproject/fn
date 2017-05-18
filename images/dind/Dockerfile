FROM docker:1.13.1-dind

RUN apk update && apk upgrade && apk add --no-cache ca-certificates

COPY entrypoint.sh /usr/local/bin/
COPY dind.sh /usr/local/bin/

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]

# USAGE: Add a CMD to your own Dockerfile to use this (NOT an ENTRYPOINT, so that this is called)
# CMD ["./runner"]
