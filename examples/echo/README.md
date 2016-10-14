This is a worker that just echoes the "input" param in the payload.

eg:

This input:

```json
{
  "input": "Yo dawg"
}
```

Will output:

```
Yo dawg
```

## Building Image

```
docker build -t iron/echo .
docker run -e 'PAYLOAD={"input": "yoooo"}' iron/echo
```
