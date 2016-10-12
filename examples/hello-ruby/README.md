This is a worker that just echoes the "input" param in the payload.

eg:

This input:

```json
{
  "name": "Johnny Utah"
}
```

Will output:

```
Hello Johnny Utah!
```

## Try it

```
echo '{"name":"Johnny"}' | docker run --rm -i iron/hello
```
