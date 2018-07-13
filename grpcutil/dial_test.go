package grpcutil

import (
	"context"
	"testing"

	"github.com/fnproject/fn/api/common"
	"google.golang.org/grpc/metadata"
)

func TestRIDFoundInMetadata(t *testing.T) {
	expected := "request-id-test"
	ctx := context.Background()
	m := make(map[string]string)
	m[common.RequestIDContextKey] = expected
	md := metadata.New(m)
	incomingCtx := metadata.NewIncomingContext(ctx, md)
	actual := ridFromMetadata(incomingCtx)
	if actual != expected {
		t.Fatalf("Wrong request ID expected '%s' got '%s'", expected, actual)
	}
}

func TestRIDNotFoundInMetadata(t *testing.T) {
	ctx := context.Background()
	m := make(map[string]string)
	md := metadata.New(m)
	incomingCtx := metadata.NewIncomingContext(ctx, md)
	actual := ridFromMetadata(incomingCtx)
	if actual != "" {
		t.Fatalf("Expected empty request ID got '%s'", actual)
	}
}
