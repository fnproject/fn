# Lambda everywhere

Lambda support for IronFunctions enables you to take your AWS Lambda functions and run them
anywhere. You should be able to take your code and run them without any changes.

## Creating Lambda Functions

Creating Lambda functions is not much different than using regular functions, just use
the `lambda-node` runtime.

```sh
fn init --runtime lambda-node <DOCKER_HUB_USERNAME>/lambda-node
```

Be sure the filename for your main handler is `func.js`.
