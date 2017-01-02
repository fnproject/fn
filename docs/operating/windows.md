# Running on Windows

Windows doesn't support Docker in Docker so you'll change the run command to the following:

```sh
docker run --rm --name functions -it -v /var/run/docker.sock:/var/run/docker.sock -v ${pwd}/data:/app/data -p 8080:8080 iron/functions
```

Then everything should work as normal. 
