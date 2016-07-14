FROM iron/dind

RUN mkdir /app
ADD functions /app/
WORKDIR /app

CMD ["./functions"]
