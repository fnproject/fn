FROM treeder/go-dind

RUN mkdir /app
ADD app /app/gateway
WORKDIR /app

ENTRYPOINT rc default && sleep 1 && ./gateway && sleep 1
