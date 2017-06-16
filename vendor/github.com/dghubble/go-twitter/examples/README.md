
# Examples

Get the dependencies and examples

    cd examples
    go get .

## User Auth (OAuth1)

A user access token (OAuth1) grants a consumer application access to a user's  Twitter resources.

Setup an OAuth1 `http.Client` with the consumer key and secret and oauth token and secret. 

    export TWITTER_CONSUMER_KEY=xxx
    export TWITTER_CONSUMER_SECRET=xxx
    export TWITTER_ACCESS_TOKEN=xxx
    export TWITTER_ACCESS_SECRET=xxx

To make requests as an application, on behalf of a user, create a `twitter` `Client` to get the home timeline, mention timeline, and more (example will **not** post Tweets).

    go run user-auth.go

## App Auth (OAuth2)

An application access token (OAuth2) allows an application to make Twitter API requests for public content, with rate limits counting against the app itself. App auth requests can be made to API endpoints which do not require a user context.

Setup an OAuth2 `http.Client` with the Twitter application access token.

    export TWITTER_APP_ACCESS_TOKEN=xxx

To make requests as an application, create a `twitter` `Client` and get public Tweets or timelines or other public content.

    go run app-auth.go

## Streaming API

A user access token (OAuth1) is required for Streaming API requests. See above.

    go run streaming.go

Hit CTRL-C to stop streaming. Uncomment different examples in code to try different streams.