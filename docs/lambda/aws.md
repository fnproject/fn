Interacting with AWS Services
=============================

The node.js and Python stacks include SDKs to interact with other AWS services.
For Java you will need to include any such SDK in the JAR file.

## Credentials

Running Lambda functions outside of AWS means that we cannot automatically get
access to other AWS resources based on Lambda subsuming the execution role
specified with the function. Instead, when using the AWS APIs inside your
Lambda function (for example, to access S3 buckets), you will need to pass
these credentials explicitly.

### Using environment variables for the credentials

The easiest way to do this is to pass the `AWS_ACCESS_KEY_ID` and
`AWS_SECRET_ACCESS_KEY` environment while creating or importing the lambda function from aws.

This can be done as follows:

```sh
export aws_access_key_id=<access-key>
export aws_secret_access_key=<secret_key>

./fn lambda create-function <user>/s3 nodejs example.run examples/s3/example.js examples/s3/example-payload.json --config aws_access_key_id --config aws_secret_access_key
```

or

```sh
./fn lambda create-function <user>/s3 nodejs example.run ../../lambda/examples/s3/example.js ../../lambda/examples/s3/example-payload.json --config aws_access_key_id=<access-key> --config aws_secret_access_key=<secret_key>
```

The various AWS SDKs will automatically pick these up.

## Example: Reading and writing to S3 Bucket

This example demonstrates modifying S3 buckets and using the included
ImageMagick tools in a node.js function. Our function will fetch an image
stored in a key specified by the event, resize it to a width of 1024px and save
it to another key.

The code for this example is located [here](../../examples/s3/example.js).

The event will look like:

```js
{
    "bucket": "iron-lambda-demo-images",
    "srcKey": "waterfall.jpg",
    "dstKey": "waterfall-1024.jpg"
}
```

The setup, imports and SDK initialization.

```js
var im = require('imagemagick');
var fs = require('fs');
var AWS = require('aws-sdk');

exports.run = function(event, context) {
  var bucketName = event['bucket']
  var srcImageKey = event['srcKey']
  var dstImageKey = event['dstKey']

  var s3 = new AWS.S3();
}
```

First we retrieve the source and write it to a local file so ImageMagick can
work with it.

```js
s3.getObject({
    Bucket: bucketName,
    Key: srcImageKey
  }, function (err, data) {

  if (err) throw err;

  var fileSrc = '/tmp/image-src.dat';
  var fileDst = '/tmp/image-dst.dat'
  fs.writeFileSync(fileSrc, data.Body)

});
```

The actual resizing involves using the identify function to get the current
size (we only resize if the image is wider than 1024px), then doing the actual
conversion to `fileDst`. Finally we upload to S3.

```js
im.identify(fileSrc, function(err, features) {
  resizeIfRequired(err, features, fileSrc, fileDst, function(err, resized) {
    if (err) throw err;
    if (resized) {
      s3.putObject({
        Bucket:bucketName,
        Key: dstImageKey,
        Body: fs.createReadStream(fileDst),
        ContentType: 'image/jpeg',
        ACL: 'public-read',
      }, function (err, data) {
        if (err) throw err;
        context.done()
      });
    } else {
      context.done();
    }
  });
});
```