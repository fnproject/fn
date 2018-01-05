// package s3 implements an s3 api compatible log store
package s3

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/url"
	"strings"

	"github.com/fnproject/fn/api/models"
	"github.com/minio/minio-go"
	"github.com/sirupsen/logrus"
)

// TODO we should encrypt these, user will have to supply a key though (or all
// OSS users logs will be encrypted with same key unless they change it which
// just seems mean...)

// TODO do we need to use the v2 API? can't find BMC object store docs :/

const (
	contentType = "text/plain"
)

type store struct {
	client *minio.Client
	bucket string
}

// s3://access_key_id:secret_access_key@host/location/bucket_name?ssl=true
// Note that access_key_id and secret_access_key must be URL encoded if they contain unsafe characters!
func New(u *url.URL) (models.LogStore, error) {
	endpoint := u.Host

	var accessKeyID, secretAccessKey string
	if u.User != nil {
		accessKeyID = u.User.Username()
		secretAccessKey, _ = u.User.Password()
	}
	useSSL := u.Query().Get("ssl") == "true"

	strs := strings.SplitN(u.Path, "/", 3)
	if len(strs) < 3 {
		return nil, errors.New("must provide bucket name and region in path of s3 api url. e.g. s3://s3.com/us-east-1/my_bucket")
	}
	location := strs[1]
	bucketName := strs[2]
	if location == "" {
		return nil, errors.New("must provide non-empty location in path of s3 api url. e.g. s3://s3.com/us-east-1/my_bucket")
	} else if bucketName == "" {
		return nil, errors.New("must provide non-empty bucket name in path of s3 api url. e.g. s3://s3.com/us-east-1/my_bucket")
	}

	logrus.WithFields(logrus.Fields{"bucketName": bucketName, "location": location, "endpoint": endpoint, "access_key_id": accessKeyID, "useSSL": useSSL}).Info("checking / creating s3 bucket")

	client, err := minio.NewWithRegion(endpoint, accessKeyID, secretAccessKey, useSSL, location)
	if err != nil {
		return nil, err
	}

	// ensure the bucket exists, creating if it does not
	err = client.MakeBucket(bucketName, location)
	if errMake := err; err != nil {
		// Check to see if we already own this bucket (which happens if you run this twice)
		exists, err := client.BucketExists(bucketName)
		if err != nil {
			return nil, err
		} else if !exists {
			return nil, errors.New("could not create bucket and bucket does not exist, please check permissions: " + errMake.Error())
		}
	}

	return &store{
		client: client,
		bucket: bucketName,
	}, nil
}

func path(appName, callID string) string {
	// raw url encode, b/c s3 does not like: & $ @ = : ; + , ?
	appName = base64.RawURLEncoding.EncodeToString([]byte(appName)) // TODO optimize..
	return appName + "/" + callID
}

func (s *store) InsertLog(ctx context.Context, appName, callID string, callLog io.Reader) error {
	objectName := path(appName, callID)
	_, err := s.client.PutObjectWithContext(ctx, s.bucket, objectName, callLog, -1, minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (s *store) GetLog(ctx context.Context, appName, callID string) (io.Reader, error) {
	objectName := path(appName, callID)
	obj, err := s.client.GetObjectWithContext(ctx, s.bucket, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, err // this is always nil, for now, thanks minio :(
	}

	_, err = obj.Stat()
	if err != nil {
		errResp := minio.ToErrorResponse(err)
		if errResp.StatusCode == 404 {
			return nil, models.ErrCallLogNotFound
		}
		return nil, err
	}

	return obj, nil
}
