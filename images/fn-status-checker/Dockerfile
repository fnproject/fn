FROM ocr-docker-remote.artifactory.oci.oraclecorp.com/os/oraclelinux:8-slim
WORKDIR /function
COPY fn-status-checker /function
ENTRYPOINT ["./fn-status-checker"]
