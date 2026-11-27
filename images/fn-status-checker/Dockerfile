FROM ocr-docker-remote.artifactory.oci.oraclecorp.com/os/oraclelinux:8-slim
RUN microdnf -y update openssl-libs && microdnf clean all
WORKDIR /function
COPY fn-status-checker /function
ENTRYPOINT ["./fn-status-checker"]
