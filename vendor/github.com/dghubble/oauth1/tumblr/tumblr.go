// Package tumblr provides constants for using OAuth 1 to access Tumblr.
package tumblr

import (
	"github.com/dghubble/oauth1"
)

// Endpoint is Tumblr's OAuth 1a endpoint.
var Endpoint = oauth1.Endpoint{
	RequestTokenURL: "http://www.tumblr.com/oauth/request_token",
	AuthorizeURL:    "http://www.tumblr.com/oauth/authorize",
	AccessTokenURL:  "http://www.tumblr.com/oauth/access_token",
}
