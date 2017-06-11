// Package xing provides constants for using OAuth1 to access Xing.
package xing

import (
	"github.com/dghubble/oauth1"
)

// Endpoint is Xing's OAuth 1 endpoint.
var Endpoint = oauth1.Endpoint{
	RequestTokenURL: "https://api.xing.com/v1/request_token",
	AuthorizeURL:    "https://api.xing.com/v1/authorize",
	AccessTokenURL:  "https://api.xing.com/v1/access_token",
}
