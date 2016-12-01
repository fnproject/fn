Import existing AWS Lambda functions
====================================

The [fn](https://github.com/iron-io/functions/fn/) tool includes a set of
commands to act on Lambda functions. Most of these are described in
[getting-started](./getting-started.md). One more subcommand is `aws-import`.

If you have an existing AWS Lambda function, you can use this command to
automatically convert it to a Docker image that is ready to be deployed on
other platforms.

### Credentials

To use this, either have your AWS access key and secret key set in config
files, or in environment variables. In addition, you'll want to set a default
region. You can use the `aws` tool to set this up. Full instructions are in the
[AWS documentation][awscli].

[awscli]: http://docs.aws.amazon.com/cli/latest/userguide/cli-chap-getting-started.html#cli-config-files

### Importing

The aws-import command is constructed as follows:

```bash
fn lambda aws-import <arn> <region> <image>
```

* arn: describes the ARN formats which uniquely identify the AWS lambda resource
* region: region on which the lambda is hosted
* image: the name of the created docker image which should have the format <username>/<image-name>

Assuming you have a lambda with the following arn `arn:aws:lambda:us-west-2:123141564251:function:my-function`, the following command:

```sh
fn lambda aws-import arn:aws:lambda:us-west-2:123141564251:function:my-function us-east-1 user/my-function
```

will import the function code from the region `us-east-1` to a directory called `./user/my-function`. Inside the directory you will find the `function.yml`, `Dockerfile`, and all the files needed for running the function.

Using Lambda with Docker Hub and IronFunctions requires that the Docker image be
named `<Docker Hub username>/<image name>`. This is used to uniquely identify
images on Docker Hub. Please use the `<Docker Hub username>/<image
name>` as the image name with `aws-import` to create a correctly named image.

If you only want to download the code, pass the `--download-only` flag. The
 `--profile` flag is available similar to the `aws` tool to help
you tweak the settings on a command level. Finally, you can import a different version of your lambda function than the latest one
by passing `--version <version>.`

You can then deploy the imported lambda as follows:
```
./fn deploy -d ./user/my-function user
````
Now the function can be reached via ```http://$HOSTNAME/r/user/my-function```