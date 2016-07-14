
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
docker run -e "IRON_TOKEN=GP8cqlKSrcpmqeR8x9WKD4qSAss" -e "IRON_PROJECT_ID=4fd2729368a0197d1102056b" -e "CLOUDFLARE_EMAIL=treeder@gmail.com" -e "CLOUDFLARE_API_KEY=X" --rm -it --privileged -p 8080:8080 iron/functions
```

Push it:

```sh
docker push iron/functions
```

Get it on a server and point router.iron.computer (on cloudflare) to the machine.

After deploying, run it with:

```sh
docker run -e --name functions -it --privileged -d -p 80:80 "IRON_TOKEN=GP8cqlKSrcpmqeR8x9WKD4qSAss" -e "IRON_PROJECT_ID=4fd2729368a0197d1102056b" -e PORT=80 iron/functions
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
