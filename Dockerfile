ARG BUILDER="builder-buildservice"
# The builder-local is only used for local development where we could build the fnserver
# in local linux container
FROM ocr-docker-remote.artifactory.oci.oraclecorp.com/os/oraclelinux:8 AS builder-local
RUN dnf -y install golang git ca-certificates && dnf clean all

WORKDIR /home/fnserver

COPY . .

RUN CGO_ENABLED=0 go build -o fnserver ./cmd/fnserver/main.go

# For build in buildserver, the artifact is copied from previous go build step.
FROM scratch AS builder-buildservice

WORKDIR /home/fnserver
COPY fnserver .

FROM ${BUILDER} AS fnserver-binary

FROM docker-remote.artifactory.oci.oraclecorp.com/docker:29-dind

RUN apk add --no-cache ca-certificates

COPY ./images/dind/preentry.sh /usr/local/bin/

ENTRYPOINT ["preentry.sh"]

WORKDIR /app

COPY --from=fnserver-binary /home/fnserver/fnserver .

CMD ["./fnserver"]
EXPOSE 8080
