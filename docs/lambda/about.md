

AWS Lambda introduced serverless computing to the masses. Wouldn't it be nice
if you could run the same Lambda functions on any platform, in any cloud?
Iron.io is proud to release a set of tools that allow just this. Package your
Lambda function in a Docker container and run it anywhere with an environment
similar to AWS Lambda.

Using a job scheduler such as IronFunctions, you can connect these functions to
webhooks and run them on-demand, at scale. You can also use a container
management system paired with a task queue to run these functions in
a self-contained, platform-independent manner.

## Use cases

Lambda functions are great for writing "worker" processes that perform some
simple, parallelizable task like image processing, ETL transformations,
asynchronous operations driven by Web APIs, or large batch processing.

All the benefits that containerization brings apply here. Our tools make it
easy to write containerized applications that will run anywhere without having
to fiddle with Docker and get the various runtimes set up. Instead you can just
write a simple function and have an "executable" ready to go.

## How does it work?

We provide base Docker images for the various runtimes that AWS Lambda
supports. The `fn` tool helps package up your Lambda function into
a Docker image layered on the base image. We provide a bootstrap script and
utilities that provide a AWS Lambda environment to your code. You can then run
the Docker image on any platform that supports Docker. This allows you to
easily move Lambda functions to any cloud provider, or host it yourself.

## Next steps

Write, package and run your Lambda functions with our [Getting started
guide](./getting-started.md). [Here is the environment](./environment.md) that
Lambda provides. `fn lambda` lists the commands to work with Lambda
functions locally.

You can [import](./import.md) existing Lambda functions hosted on Amazon!
The Docker environment required to run Lambda functions is described
[here](./docker.md).

Non-AWS Lambda functions can continue to interact with AWS services. [Working
with AWS](./aws.md) describes how to access AWS credentials, interact with
services like S3 and how to launch a Lambda function due a notification from
SNS.
