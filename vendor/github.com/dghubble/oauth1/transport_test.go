package oauth1

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTransport(t *testing.T) {
	const (
		expectedToken           = "access_token"
		expectedConsumerKey     = "consumer_key"
		expectedNonce           = "some_nonce"
		expectedSignatureMethod = "HMAC-SHA1"
		expectedTimestamp       = "123456789"
	)
	server := newMockServer(func(w http.ResponseWriter, req *http.Request) {
		params := parseOAuthParamsOrFail(t, req.Header.Get("Authorization"))
		assert.Equal(t, expectedToken, params[oauthTokenParam])
		assert.Equal(t, expectedConsumerKey, params[oauthConsumerKeyParam])
		assert.Equal(t, expectedNonce, params[oauthNonceParam])
		assert.Equal(t, expectedSignatureMethod, params[oauthSignatureMethodParam])
		assert.Equal(t, expectedTimestamp, params[oauthTimestampParam])
		assert.Equal(t, defaultOauthVersion, params[oauthVersionParam])
		// oauth_signature will vary, httptest.Server uses a random port
	})
	defer server.Close()

	config := &Config{
		ConsumerKey:    expectedConsumerKey,
		ConsumerSecret: "consumer_secret",
	}
	auther := &auther{
		config: config,
		clock:  &fixedClock{time.Unix(123456789, 0)},
		noncer: &fixedNoncer{expectedNonce},
	}
	tr := &Transport{
		source: StaticTokenSource(NewToken(expectedToken, "some_secret")),
		auther: auther,
	}
	client := &http.Client{Transport: tr}

	req, err := http.NewRequest("GET", server.URL, nil)
	assert.Nil(t, err)
	_, err = client.Do(req)
	assert.Nil(t, err)
}

func TestTransport_defaultBaseTransport(t *testing.T) {
	tr := &Transport{
		Base: nil,
	}
	assert.Equal(t, http.DefaultTransport, tr.base())
}

func TestTransport_customBaseTransport(t *testing.T) {
	expected := &http.Transport{}
	tr := &Transport{
		Base: expected,
	}
	assert.Equal(t, expected, tr.base())
}

func TestTransport_nilSource(t *testing.T) {
	tr := &Transport{
		source: nil,
		auther: &auther{
			config: &Config{},
			clock:  &fixedClock{time.Unix(123456789, 0)},
			noncer: &fixedNoncer{"any_nonce"},
		},
	}
	client := &http.Client{Transport: tr}
	resp, err := client.Get("http://example.com")
	assert.Nil(t, resp)
	if assert.Error(t, err) {
		assert.Equal(t, "Get http://example.com: oauth1: Transport's source is nil", err.Error())
	}
}

func TestTransport_emptySource(t *testing.T) {
	tr := &Transport{
		source: StaticTokenSource(nil),
		auther: &auther{
			config: &Config{},
			clock:  &fixedClock{time.Unix(123456789, 0)},
			noncer: &fixedNoncer{"any_nonce"},
		},
	}
	client := &http.Client{Transport: tr}
	resp, err := client.Get("http://example.com")
	assert.Nil(t, resp)
	if assert.Error(t, err) {
		assert.Equal(t, "Get http://example.com: oauth1: Token is nil", err.Error())
	}
}

func TestTransport_nilAuther(t *testing.T) {
	tr := &Transport{
		source: StaticTokenSource(&Token{}),
		auther: nil,
	}
	client := &http.Client{Transport: tr}
	resp, err := client.Get("http://example.com")
	assert.Nil(t, resp)
	if assert.Error(t, err) {
		assert.Equal(t, "Get http://example.com: oauth1: Transport's auther is nil", err.Error())
	}
}

func newMockServer(handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(handler))
}
