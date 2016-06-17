FROM iron/dind

RUN mkdir /app
ADD gateway /app/
WORKDIR /app

CMD ["./gateway"]
