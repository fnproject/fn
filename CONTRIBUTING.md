
## Building/Testing

Build:

```sh
# one time:
glide install
# then every time
./build.sh
```

Test it, the iron token and project id are for cache.

```sh
docker run --env-file .env --rm -it --privileged -p 8080:8080 iron/functions
```

## Releasing

```sh
./release.sh
```

## FOR INFLUX AND ANALYTICS


```sh
docker run -p 8083:8083 -p 8086:8086 \
      -v $PWD:/var/lib/influxdb \
      --rm --name influxdb \
      influxdb:alpine
```

CLI: 

```sh
docker run --rm --link influxdb -it influxdb:alpine influx -host influxdb
```

chronograf: 

```sh
# they don't have an alpine image yet chronograf
docker run -p 10000:10000 --link influxdb chronograf
```

Open UI: http://localhost:10000

Add server with host `influxdb`

