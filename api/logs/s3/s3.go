// package s3 implements an s3 api compatible log store
package s3

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
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
	client     *s3.S3
	uploader   *s3manager.Uploader
	downloader *s3manager.Downloader
	bucket     string
}

// decorator around the Reader interface that keeps track of the number of bytes read
// in order to avoid double buffering and track Reader size
type countingReader struct {
	r     io.Reader
	count int
}

func (cr *countingReader) Read(p []byte) (n int, err error) {
	n, err = cr.r.Read(p)
	cr.count += n
	return n, err
}

func createStore(bucketName, endpoint, region, accessKeyID, secretAccessKey string, useSSL bool) *store {
	config := &aws.Config{
		Credentials:      credentials.NewStaticCredentials(accessKeyID, secretAccessKey, ""),
		Endpoint:         aws.String(endpoint),
		Region:           aws.String(region),
		DisableSSL:       aws.Bool(!useSSL),
		S3ForcePathStyle: aws.Bool(true),
	}
	session := session.Must(session.NewSession(config))
	client := s3.New(session)

	return &store{
		client:     client,
		uploader:   s3manager.NewUploaderWithClient(client),
		downloader: s3manager.NewDownloaderWithClient(client),
		bucket:     bucketName,
	}
}

// s3://access_key_id:secret_access_key@host/region/bucket_name?ssl=true
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
	region := strs[1]
	bucketName := strs[2]
	if region == "" {
		return nil, errors.New("must provide non-empty region in path of s3 api url. e.g. s3://s3.com/us-east-1/my_bucket")
	} else if bucketName == "" {
		return nil, errors.New("must provide non-empty bucket name in path of s3 api url. e.g. s3://s3.com/us-east-1/my_bucket")
	}

	logrus.WithFields(logrus.Fields{"bucketName": bucketName, "region": region, "endpoint": endpoint, "access_key_id": accessKeyID, "useSSL": useSSL}).Info("checking / creating s3 bucket")
	store := createStore(bucketName, endpoint, region, accessKeyID, secretAccessKey, useSSL)

	// ensure the bucket exists, creating if it does not
	_, err := store.client.CreateBucket(&s3.CreateBucketInput{Bucket: aws.String(bucketName)})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeBucketAlreadyOwnedByYou, s3.ErrCodeBucketAlreadyExists:
				// bucket already exists, NO-OP
			default:
				return nil, fmt.Errorf("failed to create bucket %s: %s", bucketName, aerr.Message())
			}
		} else {
			return nil, fmt.Errorf("unexpected error creating bucket %s: %s", bucketName, err.Error())
		}
	}

	return store, nil
}

func path(appName, callID string) string {
	// raw url encode, b/c s3 does not like: & $ @ = : ; + , ?
	appName = base64.RawURLEncoding.EncodeToString([]byte(appName)) // TODO optimize..
	return appName + "/" + callID
}

func (s *store) InsertLog(ctx context.Context, appName, callID string, callLog io.Reader) error {
	span, ctx := tracing.StartSpan(ctx, "s3_insert_log")
	defer span.Finish()

	// wrap original reader in a decorator to keep track of read bytes without buffering
	cr := &countingReader{r: callLog}

	objectName := path(appName, callID)
	params := &s3manager.UploadInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(objectName),
		Body:        cr,
		ContentType: aws.String(contentType),
	}

	logrus.WithFields(logrus.Fields{"bucketName": s.bucket, "key": objectName}).Debug("Uploading log")
	_, err := s.uploader.UploadWithContext(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to write log, %v", err)
	}

	common.PublishHistogramToSpan(span, "s3_log_upload_size", float64(cr.count))
	return nil
}

func (s *store) GetLog(ctx context.Context, appName, callID string) (io.Reader, error) {
	span, ctx := tracing.StartSpan(ctx, "s3_get_log")
	defer span.Finish()

	objectName := path(appName, callID)
	logrus.WithFields(logrus.Fields{"bucketName": s.bucket, "key": objectName}).Debug("Downloading log")

	// stream the logs to an in-memory buffer
	target := &aws.WriteAtBuffer{}
	size, err := s.downloader.DownloadWithContext(ctx, target, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectName),
	})
	if err != nil {
		aerr, ok := err.(awserr.Error)
		if ok && aerr.Code() == s3.ErrCodeNoSuchKey {
			return nil, models.ErrCallLogNotFound
		}
		return nil, fmt.Errorf("failed to read log, %v", err)
	}

	common.PublishHistogramToSpan(span, "s3_log_download_size", float64(size))
	return bytes.NewReader(target.Bytes()), nil
}
