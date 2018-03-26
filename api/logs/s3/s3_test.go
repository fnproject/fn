package s3

import (
	"net/url"
	"os"
	"testing"

	logTesting "github.com/fnproject/fn/api/logs/testing"
)

func TestS3(t *testing.T) {
	minio := os.Getenv("MINIO_URL")
	if minio == "" {
		t.Skip("no minio specified in url, skipping (use `make test`)")
		return
	}

	uLog, err := url.Parse(minio)
	if err != nil {
		t.Fatalf("failed to parse url: %v", err)
	}

	ls, err := New(uLog)
	if err != nil {
		t.Fatalf("failed to create s3 datastore: %v", err)
	}
	logTesting.Test(t, nil, ls)
}
