FROM iron/dind

WORKDIR /app

ADD functions-alpine /app/functions

CMD ["./functions"]
