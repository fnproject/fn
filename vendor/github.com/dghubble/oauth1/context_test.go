package oauth1

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

func TestContextTransport(t *testing.T) {
	client := &http.Client{
		Transport: http.DefaultTransport,
	}
	ctx := context.WithValue(NoContext, HTTPClient, client)
	assert.Equal(t, http.DefaultTransport, contextTransport(ctx))
}

func TestContextTransport_NoContextClient(t *testing.T) {
	assert.Nil(t, contextTransport(NoContext))
}
