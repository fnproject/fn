# Applications

In `fn`, an application is a group of functions with path mappings (routes) to each function ([learn more](model.md)).
We've tried to make it easy to work with full applications by providing tools that work with all the applications functions.

## Creating an Application

All you have to do is create a file called `app.yaml` in your applications root directory, and the only required field is a name:

```yaml
name: myawesomeapp
```

Once you have that file in place, the `fn` commands will work in the context of that application.

## The Index Function (aka: Root Function)

The root app directory can also contain a `func.yaml` which will be the function access at `/`.

## Function paths

By default, the function name and path will be the same as the directory structure. For instance, if you
have a structure like this:

```txt
- app.yaml
- func.yaml
- func.go
- hello/
  - func.yaml
  - func.js
- users/
  - func.yaml
  - func.rb
```

The URL's to access those functions will be:

```
http://abc.io/ -> root function
http://abc.io/hello -> function in hello/ directory
http://abc.io/users -> function in users/ directory
```

## Deploying an entire app at once

```sh
fn deploy --all
```

If you're just testing locally, you can speed it up with the `--local` flag.

## Deploying a single function in the app

To deploy the `hello` function only, from the root dir, run:

```sh
fn deploy hello
```

## Example app

See https://github.com/fnproject/fn/tree/master/examples/apps/hellos for a simple example.
