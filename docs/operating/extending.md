# Building Custom Server with Extensions

You can easily add any number of extensions to Fn and then build your own custom image.

Simply create an `ext.yaml` file with the extensions you want added:

```yaml
extensions:
  - name: github.com/treeder/fn-ext-example/logspam
  - name: github.com/treeder/fn-ext-example/logspam2
```

Build it:

```sh
fn build-server -t imageuser/imagename
```

`-t` takes the same input as `docker build -t`, tagging your image.

Now run your new server:

```sh
docker run --rm --name fnserver -it -v /var/run/docker.sock:/var/run/docker.sock -v $PWD/data:/app/data -p 8080:8080 imageuser/imagename
```
