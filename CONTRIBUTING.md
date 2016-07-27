## Building

First time or when a dependency changes or when the API changes, run:
```
glide install
```

To quick build and run (using default database):

```sh
hack/api.sh
```

To build the docker image:

```sh
hack/build.sh
```

## Releasing

```sh
hack/release.sh
```
