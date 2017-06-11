package oauth1

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCommonOAuthParams(t *testing.T) {
	config := &Config{ConsumerKey: "some_consumer_key"}
	auther := &auther{config, &fixedClock{time.Unix(50037133, 0)}, &fixedNoncer{"some_nonce"}}
	expectedParams := map[string]string{
		"oauth_consumer_key":     "some_consumer_key",
		"oauth_signature_method": "HMAC-SHA1",
		"oauth_timestamp":        "50037133",
		"oauth_nonce":            "some_nonce",
		"oauth_version":          "1.0",
	}
	assert.Equal(t, expectedParams, auther.commonOAuthParams())
}

func TestNonce(t *testing.T) {
	auther := &auther{}
	nonce := auther.nonce()
	// assert that 32 bytes (256 bites) become 44 bytes since a base64 byte
	// zeros the 2 high bits. 3 bytes convert to 4 base64 bytes, 40 base64 bytes
	// represent the first 30 of 32 bytes, = padding adds another 4 byte group.
	// base64 bytes = 4 * floor(bytes/3) + 4
	assert.Equal(t, 44, len([]byte(nonce)))
}

func TestEpoch(t *testing.T) {
	a := &auther{}
	// assert that a real time is used by default
	assert.InEpsilon(t, time.Now().Unix(), a.epoch(), 1)
	// assert that the fixed clock can be used for testing
	a = &auther{clock: &fixedClock{time.Unix(50037133, 0)}}
	assert.Equal(t, int64(50037133), a.epoch())
}

func TestSigner_Default(t *testing.T) {
	config := &Config{ConsumerSecret: "consumer_secret"}
	a := newAuther(config)
	// echo -n "hello world" | openssl dgst -sha1 -hmac "consumer_secret&token_secret" -binary | base64
	expectedSignature := "BE0uILOruKfSXd4UzYlLJDfOq08="
	// assert that the default signer produces the expected HMAC-SHA1 digest
	method := a.signer().Name()
	digest, err := a.signer().Sign("token_secret", "hello world")
	assert.Nil(t, err)
	assert.Equal(t, "HMAC-SHA1", method)
	assert.Equal(t, expectedSignature, digest)
}

type identitySigner struct{}

func (s *identitySigner) Name() string {
	return "identity"
}

func (s *identitySigner) Sign(tokenSecret, message string) (string, error) {
	return message, nil
}

func TestSigner_Custom(t *testing.T) {
	config := &Config{
		ConsumerSecret: "consumer_secret",
		Signer:         &identitySigner{},
	}
	a := newAuther(config)
	// assert that the custom signer is used
	method := a.signer().Name()
	digest, err := a.signer().Sign("secret", "hello world")
	assert.Nil(t, err)
	assert.Equal(t, "identity", method)
	assert.Equal(t, "hello world", digest)
}

func TestAuthHeaderValue(t *testing.T) {
	cases := []struct {
		params     map[string]string
		authHeader string
	}{
		{map[string]string{}, "OAuth "},
		{map[string]string{"a": "b"}, `OAuth a="b"`},
		{map[string]string{"a": "b", "c": "d", "e": "f", "1": "2"}, `OAuth 1="2", a="b", c="d", e="f"`},
		{map[string]string{"/= +doencode": "/= +doencode"}, `OAuth %2F%3D%20%2Bdoencode="%2F%3D%20%2Bdoencode"`},
		{map[string]string{"-._~dontencode": "-._~dontencode"}, `OAuth -._~dontencode="-._~dontencode"`},
	}
	for _, c := range cases {
		assert.Equal(t, c.authHeader, authHeaderValue(c.params))
	}
}

func TestEncodeParameters(t *testing.T) {
	input := map[string]string{
		"a": "Dogs, Cats & Mice",
		"☃": "snowman",
		"ル": "ル",
	}
	expected := map[string]string{
		"a":         "Dogs%2C%20Cats%20%26%20Mice",
		"%E2%98%83": "snowman",
		"%E3%83%AB": "%E3%83%AB",
	}
	assert.Equal(t, expected, encodeParameters(input))
}

func TestSortParameters(t *testing.T) {
	input := map[string]string{
		".":         "ape",
		"5.6":       "bat",
		"rsa":       "cat",
		"%20":       "dog",
		"%E3%83%AB": "eel",
		"dup":       "fox",
		//"dup": "fix",         // duplicate keys not supported
	}
	expected := []string{
		"%20=dog",
		"%E3%83%AB=eel",
		".=ape",
		"5.6=bat",
		"dup=fox",
		"rsa=cat",
	}
	assert.Equal(t, expected, sortParameters(input, "%s=%s"))
}

func TestCollectParameters(t *testing.T) {
	// example from RFC 5849 3.4.1.3.1
	oauthParams := map[string]string{
		"oauth_token":            "kkk9d7dh3k39sjv7",
		"oauth_consumer_key":     "9djdj82h48djs9d2",
		"oauth_signature_method": "HMAC-SHA1",
		"oauth_timestamp":        "137131201",
		"oauth_nonce":            "7d8f3e4a",
	}
	values := url.Values{}
	values.Add("c2", "")
	values.Add("plus", "2 q") // duplicate keys not supported, a3 -> plus
	req, err := http.NewRequest("POST", "/request?b5=%3D%253D&a3=a&c%40=&a2=r%20b", strings.NewReader(values.Encode()))
	assert.Nil(t, err)
	req.Header.Set(contentType, formContentType)
	params, err := collectParameters(req, oauthParams)
	// assert parameters were collected from oauthParams, the query, and form body
	expected := map[string]string{
		"b5":                     "=%3D",
		"a3":                     "a",
		"c@":                     "",
		"a2":                     "r b",
		"oauth_token":            "kkk9d7dh3k39sjv7",
		"oauth_consumer_key":     "9djdj82h48djs9d2",
		"oauth_signature_method": "HMAC-SHA1",
		"oauth_timestamp":        "137131201",
		"oauth_nonce":            "7d8f3e4a",
		"c2":                     "",
		"plus":                   "2 q",
	}
	assert.Nil(t, err)
	assert.Equal(t, expected, params)
	// RFC 5849 3.4.1.3.1 requires a {"a3"="2 q"} be form encoded to "a3=2+q" in
	// the application/x-www-form-urlencoded body. The parameter "2+q" should be
	// read as "2 q" and percent encoded to "2%20q".
	// In Go, data is form encoded by calling Encode on url.Values{} (URL
	// encoding) and decoded with url.ParseQuery to url.Values. So the encoding
	// of "2 q" to "2+q" and decoding back to "2 q" is handled and then params
	// are percent encoded.
	// http://golang.org/src/net/http/client.go#L496
	// http://golang.org/src/net/http/request.go#L837
}

func TestSignatureBase(t *testing.T) {
	reqA, err := http.NewRequest("get", "HTTPS://HELLO.IO?q=test", nil)
	assert.Nil(t, err)
	reqB, err := http.NewRequest("POST", "http://hello.io:8080", nil)
	assert.Nil(t, err)
	cases := []struct {
		req           *http.Request
		params        map[string]string
		signatureBase string
	}{
		{reqA, map[string]string{"a": "b", "c": "d"}, "GET&https%3A%2F%2Fhello.io&a%3Db%26c%3Dd"},
		{reqB, map[string]string{"a": "b"}, "POST&http%3A%2F%2Fhello.io%3A8080&a%3Db"},
	}
	// assert that method is uppercased, base uri rules applied, queries added, joined by &
	for _, c := range cases {
		base := signatureBase(c.req, c.params)
		assert.Equal(t, c.signatureBase, base)
	}
}

func TestBaseURI(t *testing.T) {
	reqA, err := http.NewRequest("GET", "HTTP://EXAMPLE.COM:80/r%20v/X?id=123", nil)
	assert.Nil(t, err)
	reqB, err := http.NewRequest("POST", "https://www.example.net:8080/?q=1", nil)
	assert.Nil(t, err)
	reqC, err := http.NewRequest("POST", "https://example.com:443", nil)
	cases := []struct {
		req     *http.Request
		baseURI string
	}{
		{reqA, "http://example.com/r%20v/X"},
		{reqB, "https://www.example.net:8080/"},
		{reqC, "https://example.com"},
	}
	for _, c := range cases {
		baseURI := baseURI(c.req)
		assert.Equal(t, c.baseURI, baseURI)
	}
}

func TestNormalizedParameterString(t *testing.T) {
	simple := map[string]string{
		"a": "b & c",
		"☃": "snowman",
	}
	rfcExample := map[string]string{
		"b5":                     "=%3D",
		"a3":                     "a",
		"c@":                     "",
		"a2":                     "r b",
		"oauth_token":            "kkk9d7dh3k39sjv7",
		"oauth_consumer_key":     "9djdj82h48djs9d2",
		"oauth_signature_method": "HMAC-SHA1",
		"oauth_timestamp":        "137131201",
		"oauth_nonce":            "7d8f3e4a",
		"c2":                     "",
		"plus":                   "2 q",
	}
	cases := []struct {
		params       map[string]string
		parameterStr string
	}{
		{simple, "%E2%98%83=snowman&a=b%20%26%20c"},
		{rfcExample, "a2=r%20b&a3=a&b5=%3D%253D&c%40=&c2=&oauth_consumer_key=9djdj82h48djs9d2&oauth_nonce=7d8f3e4a&oauth_signature_method=HMAC-SHA1&oauth_timestamp=137131201&oauth_token=kkk9d7dh3k39sjv7&plus=2%20q"},
	}
	for _, c := range cases {
		assert.Equal(t, c.parameterStr, normalizedParameterString(c.params))
	}
}
