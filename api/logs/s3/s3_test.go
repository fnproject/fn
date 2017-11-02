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
		t.Skip()
		return
	}

	uLog, err := url.Parse(minio)
	if err != nil {
		t.Fatalf("failed to parse url: %v", err)
	}

	ls, err := New(uLog)
	if err != nil {
		t.Fatalf("failed to create sqlite3 datastore: %v", err)
	}
	logTesting.Test(t, ls)
}
