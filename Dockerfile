FROM treeder/go-dind

RUN mkdir /app
ADD app /app/gateway
#ADD /home/treeder/.iron.json /app/iron.json
WORKDIR /app

ENTRYPOINT rc default && sleep 1 && ./gateway && sleep 1
