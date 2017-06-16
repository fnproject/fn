// Package dropbox provides constants for using OAuth1 to access Dropbox.
package dropbox

import (
	"github.com/dghubble/oauth1"
)

// Endpoint is Dropbox's OAuth 1 endpoint.
var Endpoint = oauth1.Endpoint{
	RequestTokenURL: "https://api.dropbox.com/1/oauth/request_token",
	AuthorizeURL:    "https://api.dropbox.com/1/oauth/authorize",
	AccessTokenURL:  "https://api.dropbox.com/1/oauth/access_token",
}
