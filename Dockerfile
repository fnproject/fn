FROM docker-remote.artifactory.oci.oraclecorp.com/docker:27.3.1-dind

RUN apk add --no-cache ca-certificates

COPY ./images/dind/preentry.sh /usr/local/bin/

ENTRYPOINT ["preentry.sh"]

WORKDIR /app

COPY fnserver .

CMD ["./fnserver"]
EXPOSE 8080
