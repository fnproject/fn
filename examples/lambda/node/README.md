# Lambda Node Example

This is the exact same function that is in the AWS Lambda tutorial.

Other than a different runtime, this is no different than any other node example.

To use the lambda-nodejs4.3 runtime, use this `fn init` command:

```sh
fn init --runtime lambda-nodejs4.3 <DOCKER_HUB_USERNAME>/lambda-node
fn build
cat payload.json | fn run
```
