FROM iron/dind

RUN mkdir /app
ADD functions-alpine /app/functions
WORKDIR /app

CMD ["./functions"]
