# Building Custom Server with Extensions

## Building custom server with extensions
You can easily add any number of extensions to Fn and then build your own custom image.

Simply create an `ext.yaml` file with the extensions you want added:

```yaml
extensions:
  - name: github.com/fnproject/fn-ext-example/logspam
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

## Building custom server from source code

This method is good for extension developers as it's fast builds for dev/testing.

Copy [main.go](../../cmd/fnserver/main.go) from fn repo, then add plugins. See main.go in this repo for an example.

Then assuming you have the fn project in your GOPATH or you've vendored it here, it should build:
```bash
go build -o fnserver
./fnserver
```

Then deploy a function and you'll see the special spam output like this:
```
        ______
       / ____/___
      / /_  / __ \
     / __/ / / / /
    /_/   /_/ /_/
        v0.3.528

INFO[2018-08-08T13:46:13+03:00] Fn serving on `:8080`                         type=full
YO! This is an annoying message that will happen every time a function is called.
YO! And this is an annoying message that will happen AFTER every time a function is called.
```
