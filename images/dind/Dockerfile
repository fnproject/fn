ARG DIND_VERSION
ARG base_image=docker:${DIND_VERSION}
FROM ${base_image}

RUN apk add --no-cache ca-certificates

COPY preentry.sh /usr/local/bin/

ENTRYPOINT ["preentry.sh"]

# USAGE: Add a CMD to your own Dockerfile to use this (NOT an ENTRYPOINT), eg:
# CMD ["./runner"]
