FROM ocr-docker-remote.artifactory.oci.oraclecorp.com/os/oraclelinux:8-slim

RUN groupadd --gid 1000 fn && adduser --uid 1000 --gid fn fn
WORKDIR /function
COPY fn-test-utils /function
ENTRYPOINT ["./fn-test-utils"]
