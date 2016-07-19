
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
