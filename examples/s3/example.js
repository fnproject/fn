var im = require('imagemagick');
var fs = require('fs');
var AWS = require('aws-sdk');

// cb(err, resized) is called with true if resized.
function resizeIfRequired(err, features, fileSrc, fileDst, cb) {
  if (err) {
    cb(err, false);
    return;
  }

  var targetWidth = 1024;
  if (features.width > targetWidth)
  {
    im.resize({
      srcPath : fileSrc,
      dstPath : fileDst,
      width : targetWidth,
      format: 'jpg'
    }, function(err) {
      if (err) {
        cb(err, false);
      } else {
        cb(null, true);
      }
    });
  } else {
    cb(null, false);
  }
}

exports.run = function(event, context) {
  var bucketName = event['bucket']
  var srcImageKey = event['srcKey']
  var dstImageKey = event['dstKey']

  var s3 = new AWS.S3();

  s3.getObject({
      Bucket: bucketName,
      Key: srcImageKey
    }, function (err, data) {

    if (err) throw err;

    var fileSrc = '/tmp/image-src.dat';
    var fileDst = '/tmp/image-dst.dat'
    fs.writeFileSync(fileSrc, data.Body)

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
            context.succeed("Image updated");
          });
        } else {
          context.succeed("Image not updated");
        }
      });
    });
  });
}
