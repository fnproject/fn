This is a worker that we can use to check inputs to the job, such as env vars. 

Pass in checks via the payload:

```json
{
  "env_vars": {
    "foo": "bar"
  }
}
```

That will check that there is an env var called foo with the value bar passed to the task. 

## Building Image

Install [dj](https://github.com/treeder/dj/), then run:

```
docker build -t iron/checker .
docker run -e 'PAYLOAD={"env_vars": {"FOO": "bar"}}' -e "FOO=bar" iron/checker
```
