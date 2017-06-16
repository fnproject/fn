package twitter

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

var errAPI = APIError{
	Errors: []ErrorDetail{
		ErrorDetail{Message: "Status is a duplicate", Code: 187},
	},
}
var errHTTP = fmt.Errorf("unknown host")

func TestAPIError_Error(t *testing.T) {
	err := APIError{}
	if assert.Error(t, err) {
		assert.Equal(t, "", err.Error())
	}
	if assert.Error(t, errAPI) {
		assert.Equal(t, "twitter: 187 Status is a duplicate", errAPI.Error())
	}
}

func TestAPIError_Empty(t *testing.T) {
	err := APIError{}
	assert.True(t, err.Empty())
	assert.False(t, errAPI.Empty())
}

func TestRelevantError(t *testing.T) {
	cases := []struct {
		httpError error
		apiError  APIError
		expected  error
	}{
		{nil, APIError{}, nil},
		{nil, errAPI, errAPI},
		{errHTTP, APIError{}, errHTTP},
		{errHTTP, errAPI, errHTTP},
	}
	for _, c := range cases {
		err := relevantError(c.httpError, c.apiError)
		assert.Equal(t, c.expected, err)
	}
}
