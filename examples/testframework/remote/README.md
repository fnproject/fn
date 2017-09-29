# Example of Fn test framework - running functions remotely

This example will show you how to run a test suite on a function.

```sh
# build the test image (username/functions-testframework:0.0.1)
fn build
# push it
fn push
# create a route for the testframework
fn routes create testframework
# test it
fn test --remote testframework
```