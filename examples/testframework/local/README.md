# Example of IronFunctions test framework - running functions locally

This example will show you how to run a test suite on a function.

```sh
# build the test image (testframework:0.0.1)
fn build
# test it
fn test
```

Alternatively, you can force a rebuild before the test suite with:
```sh
# build and test it
fn test -b
```