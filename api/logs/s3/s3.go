// Package s3 implements an s3 api compatible log store
package s3

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/models"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opencensus.io/trace"
)

// TODO we should encrypt these, user will have to supply a key though (or all
// OSS users logs will be encrypted with same key unless they change it which
// just seems mean...)

// TODO do we need to use the v2 API? can't find BMC object store docs :/

const (
	// key prefixes
	callKeyPrefix    = "c/"
	callMarkerPrefix = "m/"
	logKeyPrefix     = "l/"
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
	client := s3.New(session.Must(session.NewSession(config)))

	return &store{
		client:     client,
		uploader:   s3manager.NewUploaderWithClient(client),
		downloader: s3manager.NewDownloaderWithClient(client),
		bucket:     bucketName,
	}
}

// New returns an s3 api compatible log store.
// url format: s3://access_key_id:secret_access_key@host/region/bucket_name?ssl=true
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

func (s *store) InsertLog(ctx context.Context, appID, callID string, callLog io.Reader) error {
	ctx, span := trace.StartSpan(ctx, "s3_insert_log")
	defer span.End()

	// wrap original reader in a decorator to keep track of read bytes without buffering
	cr := &countingReader{r: callLog}
	objectName := logKey(appID, callID)
	params := &s3manager.UploadInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(objectName),
		Body:        cr,
		ContentType: aws.String("text/plain"),
	}

	logrus.WithFields(logrus.Fields{"bucketName": s.bucket, "key": objectName}).Debug("Uploading log")
	_, err := s.uploader.UploadWithContext(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to write log, %v", err)
	}

	stats.Record(ctx, uploadSizeMeasure.M(int64(cr.count)))
	return nil
}

func (s *store) GetLog(ctx context.Context, appID, callID string) (io.Reader, error) {
	ctx, span := trace.StartSpan(ctx, "s3_get_log")
	defer span.End()

	objectName := logKey(appID, callID)
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

	stats.Record(ctx, downloadSizeMeasure.M(size))
	return bytes.NewReader(target.Bytes()), nil
}

func (s *store) InsertCall(ctx context.Context, call *models.Call) error {
	ctx, span := trace.StartSpan(ctx, "s3_insert_call")
	defer span.End()

	byts, err := json.Marshal(call)
	if err != nil {
		return err
	}

	objectName := callKey(call.AppID, call.ID)
	params := &s3manager.UploadInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(objectName),
		Body:        bytes.NewReader(byts),
		ContentType: aws.String("text/plain"),
	}

	logrus.WithFields(logrus.Fields{"bucketName": s.bucket, "key": objectName}).Debug("Uploading call")
	_, err = s.uploader.UploadWithContext(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to insert call, %v", err)
	}

	// at this point, they can point lookup the log and it will work. now, we can try to upload
	// the marker key. if the marker key upload fails, the user will simply not
	// see this entry when listing only when specifying a route path. (NOTE: this
	// behavior will go away if we stop listing by route -> triggers)

	objectName = callMarkerKey(call.AppID, call.Path, call.ID)
	params = &s3manager.UploadInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(objectName),
		Body:        bytes.NewReader([]byte{}),
		ContentType: aws.String("text/plain"),
	}

	logrus.WithFields(logrus.Fields{"bucketName": s.bucket, "key": objectName}).Debug("Uploading call marker")
	_, err = s.uploader.UploadWithContext(ctx, params)
	if err != nil {
		// XXX(reed): we could just log this?
		return fmt.Errorf("failed to write marker key for log, %v", err)
	}

	return nil
}

// GetCall returns a call at a certain id and app name.
func (s *store) GetCall(ctx context.Context, appID, callID string) (*models.Call, error) {
	ctx, span := trace.StartSpan(ctx, "s3_get_call")
	defer span.End()

	objectName := callKey(appID, callID)
	logrus.WithFields(logrus.Fields{"bucketName": s.bucket, "key": objectName}).Debug("Downloading call")

	return s.getCallByKey(ctx, objectName)
}

func (s *store) getCallByKey(ctx context.Context, key string) (*models.Call, error) {
	// stream the logs to an in-memory buffer
	var target aws.WriteAtBuffer
	_, err := s.downloader.DownloadWithContext(ctx, &target, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		aerr, ok := err.(awserr.Error)
		if ok && aerr.Code() == s3.ErrCodeNoSuchKey {
			return nil, models.ErrCallNotFound
		}
		return nil, fmt.Errorf("failed to read log, %v", err)
	}

	var call models.Call
	err = json.Unmarshal(target.Bytes(), &call)
	if err != nil {
		return nil, err
	}

	return &call, nil
}

func flipCursor(oid string) string {
	if oid == "" {
		return ""
	}

	return id.EncodeDescending(oid)
}

func callMarkerKey(app, path, id string) string {
	id = flipCursor(id)
	// s3 urls use / and are url, we need to encode this since paths have / in them
	// NOTE: s3 urls are max of 1024 chars. path is the only non-fixed sized object in here
	// but it is fixed to 256 chars in sql (by chance, mostly). further validation may be needed if weirdness ensues.
	path = base64.RawURLEncoding.EncodeToString([]byte(path))
	return callMarkerPrefix + app + "/" + path + "/" + id
}

func callKey(app, id string) string {
	id = flipCursor(id)
	return callKeyFlipped(app, id)
}

func callKeyFlipped(app, id string) string {
	return callKeyPrefix + app + "/" + id
}

func logKey(appID, callID string) string {
	return logKeyPrefix + appID + "/" + callID
}

// GetCalls returns a list of calls that satisfy the given CallFilter. If no
// calls exist, an empty list and a nil error are returned.
// NOTE: this relies on call ids being lexicographically sortable and <= 16 byte
func (s *store) GetCalls(ctx context.Context, filter *models.CallFilter) ([]*models.Call, error) {
	ctx, span := trace.StartSpan(ctx, "s3_get_calls")
	defer span.End()

	if filter.AppID == "" {
		return nil, errors.New("s3 store does not support listing across all apps")
	}

	// NOTE:
	// if filter.Path != ""
	//   find marker from marker keys, start there, list keys, get next marker from there
	// else
	//   use marker for keys

	// NOTE we need marker keys to support (app is REQUIRED):
	// 1) quick iteration per path
	// 2) sorted by id across all path
	// marker key: m : {app} : {path} : {id}
	// key: s: {app} : {id}
	//
	// also s3 api returns sorted in lexicographic order, we need the reverse of this.

	// marker is either a provided marker, or a key we create based on parameters
	// that contains app_id, may be a marker key if path is provided, and may
	// have a time guesstimate if to time is provided.

	var marker string

	// filter.Cursor is a call id, translate to our key format. if a path is
	// provided, we list keys from markers instead.
	if filter.Cursor != "" {
		marker = callKey(filter.AppID, filter.Cursor)
		if filter.Path != "" {
			marker = callMarkerKey(filter.AppID, filter.Path, filter.Cursor)
		}
	} else if t := time.Time(filter.ToTime); !t.IsZero() {
		// get a fake id that has the most significant bits set to the to_time (first 48 bits)
		fako := id.NewWithTime(t)
		//var buf [id.EncodedSize]byte
		//fakoId.MarshalTextTo(buf)
		//mid := string(buf[:10])
		mid := fako.String()
		marker = callKey(filter.AppID, mid)
		if filter.Path != "" {
			marker = callMarkerKey(filter.AppID, filter.Path, mid)
		}
	}

	// prefix prevents leaving bounds of app or path marker keys
	prefix := callKey(filter.AppID, "")
	if filter.Path != "" {
		prefix = callMarkerKey(filter.AppID, filter.Path, "")
	}

	input := &s3.ListObjectsInput{
		Bucket:  aws.String(s.bucket),
		MaxKeys: aws.Int64(int64(filter.PerPage)),
		Marker:  aws.String(marker),
		Prefix:  aws.String(prefix),
	}

	result, err := s.client.ListObjects(input)
	if err != nil {
		return nil, fmt.Errorf("failed to list logs: %v", err)
	}

	calls := make([]*models.Call, 0, len(result.Contents))

	for _, obj := range result.Contents {
		if len(calls) == filter.PerPage {
			break
		}

		// extract the app and id from the key to lookup the object, this also
		// validates we aren't reading strangely keyed objects from the bucket.
		var app, id string
		if filter.Path != "" {
			fields := strings.Split(*obj.Key, "/")
			if len(fields) != 4 {
				return calls, fmt.Errorf("invalid key in call markers: %v", *obj.Key)
			}
			app = fields[1]
			id = fields[3]
		} else {
			fields := strings.Split(*obj.Key, "/")
			if len(fields) != 3 {
				return calls, fmt.Errorf("invalid key in calls: %v", *obj.Key)
			}
			app = fields[1]
			id = fields[2]
		}

		// the id here is already reverse encoded, keep it that way.
		objectName := callKeyFlipped(app, id)

		// NOTE: s3 doesn't have a way to get multiple objects so just use GetCall
		// TODO we should reuse the buffer to decode these
		call, err := s.getCallByKey(ctx, objectName)
		if err != nil {
			common.Logger(ctx).WithError(err).WithFields(logrus.Fields{"app": app, "id": id}).Error("error filling call object")
			continue
		}

		// ensure: from_time < created_at < to_time
		fromTime := time.Time(filter.FromTime).Truncate(time.Millisecond)
		if !fromTime.IsZero() && !fromTime.Before(time.Time(call.CreatedAt)) {
			// NOTE could break, ids and created_at aren't necessarily in perfect order
			continue
		}

		toTime := time.Time(filter.ToTime).Truncate(time.Millisecond)
		if !toTime.IsZero() && !time.Time(call.CreatedAt).Before(toTime) {
			continue
		}

		calls = append(calls, call)
	}

	return calls, nil
}

func (s *store) Close() error {
	return nil
}

const (
	uploadSizeMetricName   = "s3_log_upload_size"
	downloadSizeMetricName = "s3_log_download_size"
)

var (
	uploadSizeMeasure   = stats.Int64(uploadSizeMetricName, "uploaded log size", "byte")
	downloadSizeMeasure = stats.Int64(downloadSizeMetricName, "downloaded log size", "byte")
)

// RegisterViews registers views for s3 measures
func RegisterViews(tagKeys []string) {
	err := view.Register(
		createView(uploadSizeMeasure, view.Distribution(), tagKeys),
		createView(downloadSizeMeasure, view.Distribution(), tagKeys),
	)
	if err != nil {
		logrus.WithError(err).Fatal("cannot create view")
	}
}

func createView(measure stats.Measure, agg *view.Aggregation, tagKeys []string) *view.View {
	return &view.View{
		Name:        measure.Name(),
		Description: measure.Description(),
		Measure:     measure,
		TagKeys:     makeKeys(tagKeys),
		Aggregation: agg,
	}
}

func makeKeys(names []string) []tag.Key {
	tagKeys := make([]tag.Key, len(names))
	for i, name := range names {
		key, err := tag.NewKey(name)
		if err != nil {
			logrus.Fatal(err)
		}
		tagKeys[i] = key
	}
	return tagKeys
}
