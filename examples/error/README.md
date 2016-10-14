This is a worker that errors out (ie: exits with non-zero exit code).


## Building Image

```
docker build -t iron/error .
docker run -e 'PAYLOAD={"input": "yoooo"}' iron/error
```
