package oauth1

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	expectedVersion         = "1.0"
	expectedSignatureMethod = "HMAC-SHA1"
)

func TestTwitterRequestTokenAuthHeader(t *testing.T) {
	// example from https://dev.twitter.com/web/sign-in/implementing
	var unixTimestamp int64 = 1318467427
	expectedConsumerKey := "cChZNFj6T5R0TigYB9yd1w"
	expectedCallback := "http%3A%2F%2Flocalhost%2Fsign-in-with-twitter%2F"
	expectedSignature := "F1Li3tvehgcraF8DMJ7OyxO4w9Y%3D"
	expectedTimestamp := "1318467427"
	expectedNonce := "ea9ec8429b68d6b77cd5600adbbb0456"
	config := &Config{
		ConsumerKey:    expectedConsumerKey,
		ConsumerSecret: "L8qq9PZyRg6ieKGEKhZolGC0vJWLw8iEJ88DRdyOg",
		CallbackURL:    "http://localhost/sign-in-with-twitter/",
		Endpoint: Endpoint{
			RequestTokenURL: "https://api.twitter.com/oauth/request_token",
			AuthorizeURL:    "https://api.twitter.com/oauth/authorize",
			AccessTokenURL:  "https://api.twitter.com/oauth/access_token",
		},
	}

	auther := &auther{config, &fixedClock{time.Unix(unixTimestamp, 0)}, &fixedNoncer{expectedNonce}}
	req, err := http.NewRequest("POST", config.Endpoint.RequestTokenURL, nil)
	assert.Nil(t, err)
	err = auther.setRequestTokenAuthHeader(req)
	// assert the request for a request token is signed and has an oauth_callback
	assert.Nil(t, err)
	params := parseOAuthParamsOrFail(t, req.Header.Get(authorizationHeaderParam))
	assert.Equal(t, expectedCallback, params[oauthCallbackParam])
	assert.Equal(t, expectedSignature, params[oauthSignatureParam])
	// additional OAuth parameters
	assert.Equal(t, expectedConsumerKey, params[oauthConsumerKeyParam])
	assert.Equal(t, expectedNonce, params[oauthNonceParam])
	assert.Equal(t, expectedTimestamp, params[oauthTimestampParam])
	assert.Equal(t, expectedVersion, params[oauthVersionParam])
	assert.Equal(t, expectedSignatureMethod, params[oauthSignatureMethodParam])
}

func TestTwitterAccessTokenAuthHeader(t *testing.T) {
	// example from https://dev.twitter.com/web/sign-in/implementing
	var unixTimestamp int64 = 1318467427
	expectedConsumerKey := "cChZNFj6T5R0TigYB9yd1w"
	expectedRequestToken := "NPcudxy0yU5T3tBzho7iCotZ3cnetKwcTIRlX0iwRl0"
	requestTokenSecret := "veNRnAWe6inFuo8o2u8SLLZLjolYDmDP7SzL0YfYI"
	expectedVerifier := "uw7NjWHT6OJ1MpJOXsHfNxoAhPKpgI8BlYDhxEjIBY"
	expectedSignature := "39cipBtIOHEEnybAR4sATQTpl2I%3D"
	expectedTimestamp := "1318467427"
	expectedNonce := "a9900fe68e2573b27a37f10fbad6a755"
	config := &Config{
		ConsumerKey:    expectedConsumerKey,
		ConsumerSecret: "L8qq9PZyRg6ieKGEKhZolGC0vJWLw8iEJ88DRdyOg",
		Endpoint: Endpoint{
			RequestTokenURL: "https://api.twitter.com/oauth/request_token",
			AuthorizeURL:    "https://api.twitter.com/oauth/authorize",
			AccessTokenURL:  "https://api.twitter.com/oauth/access_token",
		},
	}

	auther := &auther{config, &fixedClock{time.Unix(unixTimestamp, 0)}, &fixedNoncer{expectedNonce}}
	req, err := http.NewRequest("POST", config.Endpoint.AccessTokenURL, nil)
	assert.Nil(t, err)
	err = auther.setAccessTokenAuthHeader(req, expectedRequestToken, requestTokenSecret, expectedVerifier)
	// assert the request for an access token is signed and has an oauth_token and verifier
	assert.Nil(t, err)
	params := parseOAuthParamsOrFail(t, req.Header.Get(authorizationHeaderParam))
	assert.Equal(t, expectedRequestToken, params[oauthTokenParam])
	assert.Equal(t, expectedVerifier, params[oauthVerifierParam])
	assert.Equal(t, expectedSignature, params[oauthSignatureParam])
	// additional OAuth parameters
	assert.Equal(t, expectedConsumerKey, params[oauthConsumerKeyParam])
	assert.Equal(t, expectedNonce, params[oauthNonceParam])
	assert.Equal(t, expectedTimestamp, params[oauthTimestampParam])
	assert.Equal(t, expectedVersion, params[oauthVersionParam])
	assert.Equal(t, expectedSignatureMethod, params[oauthSignatureMethodParam])
}

// example from https://dev.twitter.com/oauth/overview/authorizing-requests,
// https://dev.twitter.com/oauth/overview/creating-signatures, and
// https://dev.twitter.com/oauth/application-only
var unixTimestampOfRequest int64 = 1318622958
var expectedTwitterConsumerKey = "xvz1evFS4wEEPTGEFPHBog"
var expectedTwitterOAuthToken = "370773112-GmHxMAgYyLbNEtIKZeRNFsMKPR9EyMZeS9weJAEb"
var expectedNonce = "kYjzVBB8Y0ZFabxSWbWovY3uYSQ2pTgmZeNu2VS4cg"
var twitterConfig = &Config{
	ConsumerKey:    expectedTwitterConsumerKey,
	ConsumerSecret: "kAcSOqF21Fu85e7zjz7ZN2U4ZRhfV3WpwPAoE3Z7kBw",
	Endpoint: Endpoint{
		RequestTokenURL: "https://api.twitter.com/oauth/request_token",
		AuthorizeURL:    "https://api.twitter.com/oauth/authorize",
		AccessTokenURL:  "https://api.twitter.com/oauth/access_token",
	},
}

func TestTwitterParameterString(t *testing.T) {
	auther := &auther{twitterConfig, &fixedClock{time.Unix(unixTimestampOfRequest, 0)}, &fixedNoncer{expectedNonce}}
	values := url.Values{}
	values.Add("status", "Hello Ladies + Gentlemen, a signed OAuth request!")
	// note: the reference example is old and uses api v1 in the URL
	req, err := http.NewRequest("post", "https://api.twitter.com/1/statuses/update.json?include_entities=true", strings.NewReader(values.Encode()))
	assert.Nil(t, err)
	req.Header.Set(contentType, formContentType)
	oauthParams := auther.commonOAuthParams()
	oauthParams[oauthTokenParam] = expectedTwitterOAuthToken
	params, err := collectParameters(req, oauthParams)
	// assert that the parameter string matches the reference
	expectedParameterString := "include_entities=true&oauth_consumer_key=xvz1evFS4wEEPTGEFPHBog&oauth_nonce=kYjzVBB8Y0ZFabxSWbWovY3uYSQ2pTgmZeNu2VS4cg&oauth_signature_method=HMAC-SHA1&oauth_timestamp=1318622958&oauth_token=370773112-GmHxMAgYyLbNEtIKZeRNFsMKPR9EyMZeS9weJAEb&oauth_version=1.0&status=Hello%20Ladies%20%2B%20Gentlemen%2C%20a%20signed%20OAuth%20request%21"
	assert.Nil(t, err)
	assert.Equal(t, expectedParameterString, normalizedParameterString(params))
}

func TestTwitterSignatureBase(t *testing.T) {
	auther := &auther{twitterConfig, &fixedClock{time.Unix(unixTimestampOfRequest, 0)}, &fixedNoncer{expectedNonce}}
	values := url.Values{}
	values.Add("status", "Hello Ladies + Gentlemen, a signed OAuth request!")
	// note: the reference example is old and uses api v1 in the URL
	req, err := http.NewRequest("post", "https://api.twitter.com/1/statuses/update.json?include_entities=true", strings.NewReader(values.Encode()))
	assert.Nil(t, err)
	req.Header.Set(contentType, formContentType)
	oauthParams := auther.commonOAuthParams()
	oauthParams[oauthTokenParam] = expectedTwitterOAuthToken
	params, err := collectParameters(req, oauthParams)
	signatureBase := signatureBase(req, params)
	// assert that the signature base string matches the reference
	// checks that method is uppercased, url is encoded, parameter string is added, all joined by &
	expectedSignatureBase := "POST&https%3A%2F%2Fapi.twitter.com%2F1%2Fstatuses%2Fupdate.json&include_entities%3Dtrue%26oauth_consumer_key%3Dxvz1evFS4wEEPTGEFPHBog%26oauth_nonce%3DkYjzVBB8Y0ZFabxSWbWovY3uYSQ2pTgmZeNu2VS4cg%26oauth_signature_method%3DHMAC-SHA1%26oauth_timestamp%3D1318622958%26oauth_token%3D370773112-GmHxMAgYyLbNEtIKZeRNFsMKPR9EyMZeS9weJAEb%26oauth_version%3D1.0%26status%3DHello%2520Ladies%2520%252B%2520Gentlemen%252C%2520a%2520signed%2520OAuth%2520request%2521"
	assert.Nil(t, err)
	assert.Equal(t, expectedSignatureBase, signatureBase)
}

func TestTwitterRequestAuthHeader(t *testing.T) {
	oauthTokenSecret := "LswwdoUaIvS8ltyTt5jkRh4J50vUPVVHtR2YPi5kE"
	expectedSignature := PercentEncode("tnnArxj06cWHq44gCs1OSKk/jLY=")
	expectedTimestamp := "1318622958"

	auther := &auther{twitterConfig, &fixedClock{time.Unix(unixTimestampOfRequest, 0)}, &fixedNoncer{expectedNonce}}
	values := url.Values{}
	values.Add("status", "Hello Ladies + Gentlemen, a signed OAuth request!")

	accessToken := &Token{expectedTwitterOAuthToken, oauthTokenSecret}
	req, err := http.NewRequest("POST", "https://api.twitter.com/1/statuses/update.json?include_entities=true", strings.NewReader(values.Encode()))
	assert.Nil(t, err)
	req.Header.Set(contentType, formContentType)
	err = auther.setRequestAuthHeader(req, accessToken)
	// assert that request is signed and has an access token token
	assert.Nil(t, err)
	params := parseOAuthParamsOrFail(t, req.Header.Get(authorizationHeaderParam))
	assert.Equal(t, expectedTwitterOAuthToken, params[oauthTokenParam])
	assert.Equal(t, expectedSignature, params[oauthSignatureParam])
	// additional OAuth parameters
	assert.Equal(t, expectedTwitterConsumerKey, params[oauthConsumerKeyParam])
	assert.Equal(t, expectedNonce, params[oauthNonceParam])
	assert.Equal(t, expectedSignatureMethod, params[oauthSignatureMethodParam])
	assert.Equal(t, expectedTimestamp, params[oauthTimestampParam])
	assert.Equal(t, expectedVersion, params[oauthVersionParam])
}

func parseOAuthParamsOrFail(t *testing.T, authHeader string) map[string]string {
	if !strings.HasPrefix(authHeader, authorizationPrefix) {
		assert.Fail(t, fmt.Sprintf("Expected Authorization header to start with \"%s\", got \"%s\"", authorizationPrefix, authHeader[:len(authorizationPrefix)+1]))
	}
	params := map[string]string{}
	for _, pairStr := range strings.Split(authHeader[len(authorizationPrefix):], ", ") {
		pair := strings.Split(pairStr, "=")
		if len(pair) != 2 {
			assert.Fail(t, "Error parsing OAuth parameter %s", pairStr)
		}
		params[pair[0]] = strings.Replace(pair[1], "\"", "", -1)
	}
	return params
}

type fixedClock struct {
	now time.Time
}

func (c *fixedClock) Now() time.Time {
	return c.now
}

type fixedNoncer struct {
	nonce string
}

func (n *fixedNoncer) Nonce() string {
	return n.nonce
}
