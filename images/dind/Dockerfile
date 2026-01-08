# This dockerfile is used for local development. Our pipeline does not build multi-arch version of this image
ARG DIND_VERSION=24.0.9-dind
FROM docker:${DIND_VERSION}

RUN apk add --no-cache ca-certificates

COPY preentry.sh /usr/local/bin/

ENTRYPOINT ["preentry.sh"]

# USAGE: Add a CMD to your own Dockerfile to use this (NOT an ENTRYPOINT), eg:
# CMD ["./runner"]
