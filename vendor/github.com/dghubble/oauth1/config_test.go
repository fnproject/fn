package oauth1

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

const expectedVerifier = "some_verifier"

func TestNewConfig(t *testing.T) {
	expectedConsumerKey := "consumer_key"
	expectedConsumerSecret := "consumer_secret"
	config := NewConfig(expectedConsumerKey, expectedConsumerSecret)
	assert.Equal(t, expectedConsumerKey, config.ConsumerKey)
	assert.Equal(t, expectedConsumerSecret, config.ConsumerSecret)
}

func TestNewClient(t *testing.T) {
	expectedToken := "access_token"
	expectedConsumerKey := "consumer_key"
	config := NewConfig(expectedConsumerKey, "consumer_secret")
	token := NewToken(expectedToken, "access_secret")
	client := config.Client(NoContext, token)

	server := newMockServer(func(w http.ResponseWriter, req *http.Request) {
		assert.Equal(t, "GET", req.Method)
		params := parseOAuthParamsOrFail(t, req.Header.Get(authorizationHeaderParam))
		assert.Equal(t, expectedToken, params[oauthTokenParam])
		assert.Equal(t, expectedConsumerKey, params[oauthConsumerKeyParam])
	})
	defer server.Close()
	client.Get(server.URL)
}

func TestNewClient_DefaultTransport(t *testing.T) {
	client := NewClient(NoContext, NewConfig("t", "s"), NewToken("t", "s"))
	// assert that the client uses the DefaultTransport
	transport, ok := client.Transport.(*Transport)
	assert.True(t, ok)
	assert.Equal(t, http.DefaultTransport, transport.base())
}

func TestNewClient_ContextClientTransport(t *testing.T) {
	baseTransport := &http.Transport{}
	baseClient := &http.Client{Transport: baseTransport}
	ctx := context.WithValue(NoContext, HTTPClient, baseClient)
	client := NewClient(ctx, NewConfig("t", "s"), NewToken("t", "s"))
	// assert that the client uses the ctx client's Transport as its base RoundTripper
	transport, ok := client.Transport.(*Transport)
	assert.True(t, ok)
	assert.Equal(t, baseTransport, transport.base())
}

// newRequestTokenServer returns a new httptest.Server for an OAuth1 provider
// request token endpoint.
func newRequestTokenServer(t *testing.T, data url.Values) *httptest.Server {
	return newMockServer(func(w http.ResponseWriter, req *http.Request) {
		assert.Equal(t, "POST", req.Method)
		assert.NotEmpty(t, req.Header.Get("Authorization"))
		w.Header().Set(contentType, formContentType)
		w.Write([]byte(data.Encode()))
	})
}

// newAccessTokenServer returns a new httptest.Server for an OAuth1 provider
// access token endpoint.
func newAccessTokenServer(t *testing.T, data url.Values) *httptest.Server {
	return newMockServer(func(w http.ResponseWriter, req *http.Request) {
		assert.Equal(t, "POST", req.Method)
		assert.NotEmpty(t, req.Header.Get("Authorization"))
		params := parseOAuthParamsOrFail(t, req.Header.Get(authorizationHeaderParam))
		assert.Equal(t, expectedVerifier, params[oauthVerifierParam])
		w.Header().Set(contentType, formContentType)
		w.Write([]byte(data.Encode()))
	})
}

// newUnparseableBodyServer returns a new httptest.Server which writes
// responses with bodies that error when parsed by url.ParseQuery.
func newUnparseableBodyServer() *httptest.Server {
	return newMockServer(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set(contentType, formContentType)
		// url.ParseQuery will error, https://golang.org/src/net/url/url_test.go#L1107
		w.Write([]byte("%gh&%ij"))
	})
}

func TestConfigRequestToken(t *testing.T) {
	expectedToken := "reqest_token"
	expectedSecret := "request_secret"
	data := url.Values{}
	data.Add("oauth_token", expectedToken)
	data.Add("oauth_token_secret", expectedSecret)
	data.Add("oauth_callback_confirmed", "true")
	server := newRequestTokenServer(t, data)
	defer server.Close()

	config := &Config{
		Endpoint: Endpoint{
			RequestTokenURL: server.URL,
		},
	}
	requestToken, requestSecret, err := config.RequestToken()
	assert.Nil(t, err)
	assert.Equal(t, expectedToken, requestToken)
	assert.Equal(t, expectedSecret, requestSecret)
}

func TestConfigRequestToken_InvalidRequestTokenURL(t *testing.T) {
	config := &Config{
		Endpoint: Endpoint{
			RequestTokenURL: "http://wrong.com/oauth/request_token",
		},
	}
	requestToken, requestSecret, err := config.RequestToken()
	assert.NotNil(t, err)
	assert.Equal(t, "", requestToken)
	assert.Equal(t, "", requestSecret)
}

func TestConfigRequestToken_CallbackNotConfirmed(t *testing.T) {
	const expectedToken = "reqest_token"
	const expectedSecret = "request_secret"
	data := url.Values{}
	data.Add("oauth_token", expectedToken)
	data.Add("oauth_token_secret", expectedSecret)
	data.Add("oauth_callback_confirmed", "false")
	server := newRequestTokenServer(t, data)
	defer server.Close()

	config := &Config{
		Endpoint: Endpoint{
			RequestTokenURL: server.URL,
		},
	}
	requestToken, requestSecret, err := config.RequestToken()
	if assert.Error(t, err) {
		assert.Equal(t, "oauth1: oauth_callback_confirmed was not true", err.Error())
	}
	assert.Equal(t, "", requestToken)
	assert.Equal(t, "", requestSecret)
}

func TestConfigRequestToken_CannotParseBody(t *testing.T) {
	server := newUnparseableBodyServer()
	defer server.Close()

	config := &Config{
		Endpoint: Endpoint{
			RequestTokenURL: server.URL,
		},
	}
	requestToken, requestSecret, err := config.RequestToken()
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "invalid URL escape")
	}
	assert.Equal(t, "", requestToken)
	assert.Equal(t, "", requestSecret)
}

func TestConfigRequestToken_MissingTokenOrSecret(t *testing.T) {
	data := url.Values{}
	data.Add("oauth_token", "any_token")
	data.Add("oauth_callback_confirmed", "true")
	server := newRequestTokenServer(t, data)
	defer server.Close()

	config := &Config{
		Endpoint: Endpoint{
			RequestTokenURL: server.URL,
		},
	}
	requestToken, requestSecret, err := config.RequestToken()
	if assert.Error(t, err) {
		assert.Equal(t, "oauth1: Response missing oauth_token or oauth_token_secret", err.Error())
	}
	assert.Equal(t, "", requestToken)
	assert.Equal(t, "", requestSecret)
}

func TestAuthorizationURL(t *testing.T) {
	expectedURL := "https://api.example.com/oauth/authorize?oauth_token=a%2Frequest_token"
	config := &Config{
		Endpoint: Endpoint{
			AuthorizeURL: "https://api.example.com/oauth/authorize",
		},
	}
	url, err := config.AuthorizationURL("a/request_token")
	assert.Nil(t, err)
	if assert.NotNil(t, url) {
		assert.Equal(t, expectedURL, url.String())
	}
}

func TestAuthorizationURL_CannotParseAuthorizeURL(t *testing.T) {
	config := &Config{
		Endpoint: Endpoint{
			AuthorizeURL: "%gh&%ij",
		},
	}
	url, err := config.AuthorizationURL("any_request_token")
	assert.Nil(t, url)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "parse")
		assert.Contains(t, err.Error(), "invalid URL")
	}
}

func TestConfigAccessToken(t *testing.T) {
	expectedToken := "access_token"
	expectedSecret := "access_secret"
	data := url.Values{}
	data.Add("oauth_token", expectedToken)
	data.Add("oauth_token_secret", expectedSecret)
	server := newAccessTokenServer(t, data)
	defer server.Close()

	config := &Config{
		Endpoint: Endpoint{
			AccessTokenURL: server.URL,
		},
	}
	accessToken, accessSecret, err := config.AccessToken("request_token", "request_secret", expectedVerifier)
	assert.Nil(t, err)
	assert.Equal(t, expectedToken, accessToken)
	assert.Equal(t, expectedSecret, accessSecret)
}

func TestConfigAccessToken_InvalidAccessTokenURL(t *testing.T) {
	config := &Config{
		Endpoint: Endpoint{
			AccessTokenURL: "http://wrong.com/oauth/access_token",
		},
	}
	accessToken, accessSecret, err := config.AccessToken("any_token", "any_secret", "any_verifier")
	assert.NotNil(t, err)
	assert.Equal(t, "", accessToken)
	assert.Equal(t, "", accessSecret)
}

func TestConfigAccessToken_CannotParseBody(t *testing.T) {
	server := newUnparseableBodyServer()
	defer server.Close()

	config := &Config{
		Endpoint: Endpoint{
			AccessTokenURL: server.URL,
		},
	}
	accessToken, accessSecret, err := config.AccessToken("any_token", "any_secret", "any_verifier")
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "invalid URL escape")
	}
	assert.Equal(t, "", accessToken)
	assert.Equal(t, "", accessSecret)
}

func TestConfigAccessToken_MissingTokenOrSecret(t *testing.T) {
	data := url.Values{}
	data.Add("oauth_token", "any_token")
	server := newAccessTokenServer(t, data)
	defer server.Close()

	config := &Config{
		Endpoint: Endpoint{
			AccessTokenURL: server.URL,
		},
	}
	accessToken, accessSecret, err := config.AccessToken("request_token", "request_secret", expectedVerifier)
	if assert.Error(t, err) {
		assert.Equal(t, "oauth1: Response missing oauth_token or oauth_token_secret", err.Error())
	}
	assert.Equal(t, "", accessToken)
	assert.Equal(t, "", accessSecret)
}

func TestParseAuthorizationCallback_GET(t *testing.T) {
	expectedToken := "token"
	expectedVerifier := "verifier"
	server := newMockServer(func(w http.ResponseWriter, req *http.Request) {
		assert.Equal(t, "GET", req.Method)
		// logic under test
		requestToken, verifier, err := ParseAuthorizationCallback(req)
		assert.Nil(t, err)
		assert.Equal(t, expectedToken, requestToken)
		assert.Equal(t, expectedVerifier, verifier)
	})
	defer server.Close()

	// OAuth1 provider calls callback url
	url, err := url.Parse(server.URL)
	assert.Nil(t, err)
	query := url.Query()
	query.Add("oauth_token", expectedToken)
	query.Add("oauth_verifier", expectedVerifier)
	url.RawQuery = query.Encode()
	http.Get(url.String())
}

func TestParseAuthorizationCallback_POST(t *testing.T) {
	expectedToken := "token"
	expectedVerifier := "verifier"
	server := newMockServer(func(w http.ResponseWriter, req *http.Request) {
		assert.Equal(t, "POST", req.Method)
		// logic under test
		requestToken, verifier, err := ParseAuthorizationCallback(req)
		assert.Nil(t, err)
		assert.Equal(t, expectedToken, requestToken)
		assert.Equal(t, expectedVerifier, verifier)
	})
	defer server.Close()

	// OAuth1 provider calls callback url
	form := url.Values{}
	form.Add("oauth_token", expectedToken)
	form.Add("oauth_verifier", expectedVerifier)
	http.PostForm(server.URL, form)
}

func TestParseAuthorizationCallback_MissingTokenOrVerifier(t *testing.T) {
	server := newMockServer(func(w http.ResponseWriter, req *http.Request) {
		assert.Equal(t, "GET", req.Method)
		// logic under test
		requestToken, verifier, err := ParseAuthorizationCallback(req)
		if assert.Error(t, err) {
			assert.Equal(t, "oauth1: Request missing oauth_token or oauth_verifier", err.Error())
		}
		assert.Equal(t, "", requestToken)
		assert.Equal(t, "", verifier)
	})
	defer server.Close()

	// OAuth1 provider calls callback url
	url, err := url.Parse(server.URL)
	assert.Nil(t, err)
	query := url.Query()
	query.Add("oauth_token", "any_token")
	query.Add("oauth_verifier", "") // missing oauth_verifier
	url.RawQuery = query.Encode()
	http.Get(url.String())
}
