# build stage
FROM golang:alpine AS build-env
ADD . /src
RUN cd /src && go build -o goapp

# final stage
FROM funcy/base
WORKDIR /app
COPY --from=build-env /src/goapp /app/
ENTRYPOINT ./goapp
