This is a worker that just echoes the "input" param in the payload.

eg:

This input:

```json
{
  "sleep": 5
}
```

Will make this container sleep for 5 seconds.


## Building Image

Install [dj](https://github.com/treeder/dj/), then run:

```
docker build -t iron/sleeper .
docker run -e 'PAYLOAD={"sleep": 5}' iron/sleeper
```
