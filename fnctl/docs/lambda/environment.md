# Environment

The base images strive to provide the same environment that AWS provides to
Lambda functions. This page describes it and any incompatibilities between AWS
Lambda and Dockerized Lambda.

## Request/Response

IronFunctions has sync/async communication with Request/Response workflows.
* sync: returns the result as the body of the response
* async: returns the task id in the body of the response as a json

The `context.succeed()` will not do anything with result
on node.js, nor will returning anything from a Python function.

## Paths

We do not make any compatibility efforts towards running your lambda function
in the same working directory as it would run on AWS. If your function makes
such assumptions, please rewrite it.

## nodejs

* node.js version [0.10.42][iron/node]
* ImageMagick version [6.9.3][magickv] and nodejs [wrapper 6.9.3][magickwrapperv]
* aws-sdk version [2.2.12][awsnodev]

[iron/node]: https://github.com/iron-io/dockers/blob/master/node/Dockerfile
[magickv]: https://pkgs.alpinelinux.org/package/main/x86_64/imagemagick
[magickwrapperv]: https://www.npmjs.com/package/imagemagick
[awsnodev]: https://aws.amazon.com/sdk-for-node-js/

### Event

Payloads MUST be a valid JSON object literal.

### Context object

* context.fail() does not currently truncate error logs.
* `context.functionName` is of the form of a docker image, for example
  `iron/test-function`.
* `context.functionVersion` is always the string `"$LATEST"`.
* `context.invokedFunctionArn` is not supported. Value is empty string.
* `context.memoryLimitInMB` does not reflect reality. Value is always `256`.
* `context.awsRequestId` reflects the environment variable `TASK_ID`. On local
  runs from `ironcli` this is a UUID. On IronFunctions this is the task ID.
* `logGroupName` and `logStreamName` are empty strings.
* `identity` and `clientContext` are always `null`.

### Exceptions

If your handler throws an exception, we only log the error message. There is no
`v8::CallSite` compatible stack trace yet.

## Python 2.7

* CPython [2.7.11][pythonv]
* boto3 (Python AWS SDK) [1.2.3][botov].

[pythonv]: https://hub.docker.com/r/iron/python/tags/
[botov]: https://github.com/boto/boto3/releases/tag/1.2.3

### Event

Event is always a `__dict__` and the payload MUST be a valid JSON object
literal.

### Context object

* `context.functionName` is of the form of a docker image, for example
  `iron/test-function`.
* `context.functionVersion` is always the string `"$LATEST"`.
* `context.invokedFunctionArn` is `None`.
* `context.awsRequestId` reflects the environment variable `TASK_ID` which is
  set to the task ID on IronFunctions. If TASK_ID is empty, a new UUID is used.
* `logGroupName`, `logStreamName`, `identity` and `clientContext` are `None`.

### Exceptions

If your Lambda function throws an Exception, it will not currently be logged as
a JSON object with trace information.

## Java 8

* OpenJDK Java Runtime [1.8.0][javav]

[javav]: https://hub.docker.com/r/iron/java/tags/

The Java8 runtime is significantly lacking at this piont and we **do not
recommend** using it.

### Handler types

There are some restrictions on the handler types supported.

#### Only a void return type is allowed

Since Lambda does not support request/response invocation, we explicitly
prohibit a non-void return type on the handler.

#### JSON parse error stack differences

AWS uses the Jackson parser, this project uses the GSON parser. So JSON parse
errors will have different traces.

#### Single item vs. List

Given a list handler like:

```java
public static void myHandler(List<Double> l) {
    // ...
}
```

If the payload is a single number, AWS Lambda will succeed and pass the handler
a list with a single item. This project will raise an exception.

#### Collections of POJOs

This project cannot currently deserialize a List or Map containing POJOs. For
example:

```java
public class Handler {
  public static MyPOJO {
    private String attribute;
    public void setAttribute(String a) {
      attribute = a;
    }

    public String getAttribute() {
      return attribute;
    }
  }

  public static void myHandler(List<MyPOJO> l) {
    // ...
  }
}
```

This handler invoked with the below event will fail!

```js
[{ "attribute": "value 1"}, { "attribute": "value 2" }]
```

#### Leveraging predefined types is not supported

Using the types in `aws-lambda-java-core` to [implement handlers][predef] is
untested and unsupported right now. While the package is available in your
function, we have not tried it out.

[predef]: http://docs.aws.amazon.com/lambda/latest/dg/java-handler-using-predefined-interfaces.html

### Logging

The [log4j and LambdaLogger
styles](http://docs.aws.amazon.com/lambda/latest/dg/java-logging.html) that log
to CloudWatch are not supported.

### Context object

* `context.getFunctionName()` returns a String of the form of a docker image,
  for example `iron/test-function`.
* `context.getFunctionVersion()` is always the string `"$LATEST"`.
* `context.getAwsRequestId()` reflects the environment variable `TASK_ID` which is
  set to the task ID on IronFunctions. If TASK_ID is empty, a new UUID is used.
* `getInvokedFunctionArn()`, `getLogGroupName()`, `getLogStreamName()`, `getIdentity()`, `getClientContext()`, `getLogger()` return `null`.
