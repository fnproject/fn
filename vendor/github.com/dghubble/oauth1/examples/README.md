
# OAuth1 Examples

## Twitter

### Authorization Flow (PIN-based)

An application can obtain a Twitter access `Token` for a user by requesting the user grant access via [3-legged](https://dev.twitter.com/oauth/3-legged) or [PIN-based](https://dev.twitter.com/oauth/pin-based) OAuth 1. Here is a command line example showing PIN-based authorization.

    export TWITTER_CONSUMER_KEY=xxx
    export TWITTER_CONSUMER_SECRET=xxx
    go run twitter-login.go

The OAuth 1 flow can be used to implement Login with Twitter. Upon receiving an access token in a callback handler on your server, issue a user some form of unforgeable session identifier (i.e. cookie, token). Note that web backends should use a real `CallbackURL`, "oob" is for PIN-based agents such as the command line.

### Authorized Requests

Use the access `Token` to make requests on behalf of a Twitter user.

    export TWITTER_CONSUMER_KEY=xxx
    export TWITTER_CONSUMER_SECRET=xxx
    export TWITTER_ACCESS_TOKEN=xxx
    export TWITTER_ACCESS_SECRET=xxx
    go run twitter-request.go


## Tumblr

### Authorization Flow

An application can obtain a Tumblr access `Token` to act on behalf of a user. Here is a command line example which requests permission. 

    export TUMBLR_CONSUMER_KEY=xxx
    export TUMBLR_CONSUMER_SECRET=xxx
    go run tumblr-login.go

### Authorized Requests

Use the access `Token` to make requests on behalf of a Tumblr user.

    export TUMBLR_CONSUMER_KEY=xxx
    export TUMBLR_CONSUMER_SECRET=xxx
    export TUMBLR_ACCESS_TOKEN=xxx
    export TUMBLR_ACCESS_SECRET=xxx
    go run tumblr-request.go

Note that only some Tumblr endpoints require OAuth1 signed requests, other endpoints require a special consumer key query parameter or no authorization.

