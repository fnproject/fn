# Testing the Lambda Docker images

The `test-function` subcommand can pass the correct parameters to `docker run`
to run those images with the payload and environment variables set up
correctly. If you would like more control, like mounting volumes, or adding
more environment variables this guide describes how to directly run these
images using:
```sh
docker run
```

An example of a valid `test-function` command would look as follows:
```
fn lambda test-function user/my-function --payload='{"firstName":"John", "lastName":"Yo" }'
```

## Payload

The payload is passed via stdin.
It is also possible to pass the payload by using the `payload` argument. Using it the payload is written to a random, opaque directory on the host.
The file itself is called `payload.json`. This directory is mapped to the
`/mnt` volume in the container, so that the payload is available in
`/mnt/payload.json`. This is not REQUIRED, since the actual runtimes use the
`PAYLOAD_FILE` environment variable to discover the payload location.


## Environment variables

The `TASK_ID` variable maps to the AWS Request ID. This should be set to
something unique (a UUID, or an incrementing number).

`test-function` runs a container with 300MB memory allocated to it. This same
information is available inside the container in the `TASK_MAXRAM` variable.
This value can be a number in bytes, or a number suffixed by `b`, `k`, `m`, `g`
for bytes, kilobytes, megabytes and gigabytes respectively. These are
case-insensitive.

The following variables are set for AWS compatibility:
* `AWS_LAMBDA_FUNCTION_NAME` - The name of the docker image.
* `AWS_LAMBDA_FUNCTION_VERSION` - The default is `$LATEST`, but any string is
  allowed.
* `AWS_ACCESS_KEY_ID` - Set this to the Access Key to allow the Lambda function
  to use AWS APIs.
* `AWS_SECRET_ACCESS_KEY` - Set this to the Secret Key to allow the Lambda
  function to use AWS APIs.

## Running the container

The default `test-function` can then be approximated as the following `docker
run` command:

```sh
mkdir /tmp/payload_dir
echo "<payload>" |
docker run -v /tmp/payload_dir:/mnt \
           -m 1G \
           -e TASK_ID=$RANDOM \
           -e TASK_MAXRAM=1G \
           -e AWS_LAMBDA_FUNCTION_NAME=user/fancyfunction \
           -e AWS_LAMBDA_FUNCTION_VERSION=1.0 \
           -e AWS_ACCESS_KEY_ID=<access key> \
           -e AWS_SECRET_ACCESS_KEY=<secret key> \
           --rm -it
           user/fancyfunction
```
