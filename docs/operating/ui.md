# User Interface for FN

### Run Functions UI

```sh
docker run --rm -it --link functions:api -p 4000:4000 -e "API_URL=http://api:8080" treeder/functions-ui
```

For more information, see:  https://github.com/treeder/functions-ui
