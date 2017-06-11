

# go-twitter [![Build Status](https://travis-ci.org/dghubble/go-twitter.png)](https://travis-ci.org/dghubble/go-twitter) [![GoDoc](https://godoc.org/github.com/dghubble/go-twitter?status.png)](https://godoc.org/github.com/dghubble/go-twitter)
<img align="right" src="https://storage.googleapis.com/dghubble/gopher-on-bird.png">

go-twitter is a Go client library for the [Twitter API](https://dev.twitter.com/rest/public). Check the [usage](#usage) section or try the [examples](/examples) to see how to access the Twitter API.

### Features

* Twitter REST API:
    * Accounts
    * Direct Messages
    * Favorites
    * Friends
    * Friendships
    * Followers
    * Search
    * Statuses
    * Timelines
    * Users
* Twitter Streaming API
    * Public Streams
    * User Streams
    * Site Streams
    * Firehose Streams

## Install

    go get github.com/dghubble/go-twitter/twitter

## Documentation

Read [GoDoc](https://godoc.org/github.com/dghubble/go-twitter/twitter)

## Usage

### REST API

The `twitter` package provides a `Client` for accessing the Twitter API. Here are some example requests.

```go
config := oauth1.NewConfig("consumerKey", "consumerSecret")
token := oauth1.NewToken("accessToken", "accessSecret")
httpClient := config.Client(oauth1.NoContext, token)

// Twitter client
client := twitter.NewClient(httpClient)

// Home Timeline
tweets, resp, err := client.Timelines.HomeTimeline(&twitter.HomeTimelineParams{
    Count: 20,
})

// Send a Tweet
tweet, resp, err := client.Statuses.Update("just setting up my twttr", nil)

// Status Show
tweet, resp, err := client.Statuses.Show(585613041028431872, nil)

// Search Tweets
search, resp, err := client.Search.Tweets(&twitter.SearchTweetParams{
    Query: "gopher",
})

// User Show
user, resp, err := client.Users.Show(&twitter.UserShowParams{
    ScreenName: "dghubble",
})

// Followers
followers, resp, err := client.Followers.List(&twitter.FollowerListParams{})
```

Authentication is handled by the `http.Client` passed to `NewClient` to handle user auth (OAuth1) or application auth (OAuth2). See the [Authentication](#authentication) section.

Required parameters are passed as positional arguments. Optional parameters are passed typed params structs (or nil).

## Streaming API

The Twitter Public, User, Site, and Firehose Streaming APIs can be accessed through the `Client` `StreamService` which provides methods `Filter`, `Sample`, `User`, `Site`, and `Firehose`.

Create a `Client` with an authenticated `http.Client`. All stream endpoints require a user auth context so choose an OAuth1 `http.Client`.

    client := twitter.NewClient(httpClient)

Next, request a managed `Stream` be started.

#### Filter

Filter Streams return Tweets that match one or more filtering predicates such as `Track`, `Follow`, and `Locations`.

```go
params := &twitter.StreamFilterParams{
    Track: []string{"kitten"},
    StallWarnings: twitter.Bool(true),
}
stream, err := client.Streams.Filter(params)
```

#### User

User Streams provide messages specific to the authenticate User and possibly those they follow.

```go
params := &twitter.StreamUserParams{
    With:          "followings",
    StallWarnings: twitter.Bool(true),
}
stream, err := client.Streams.User(params)
```

*Note* To see Direct Message events, your consumer application must ask Users for read/write/DM access to their account.

#### Sample

Sample Streams return a small sample of public Tweets.

```go
params := &twitter.StreamSampleParams{
    StallWarnings: twitter.Bool(true),
}
stream, err := client.Streams.Sample(params)
```

#### Site, Firehose

Site and Firehose Streams require your application to have special permissions, but their API works the same way.

### Receiving Messages

Each `Stream` maintains the connection to the Twitter Streaming API endpoint, receives messages, and sends them on the `Stream.Messages` channel.

Go channels support range iterations which allow you to read the messages which are of type `interface{}`.

```go
for message := range stream.Messages {
    fmt.Println(message)
}
```

If you run this in your main goroutine, it will receive messages forever unless the stream stops. To continue execution, receive messages using a separate goroutine.

### Demux

Receiving messages of type `interface{}` isn't very nice, it means you'll have to type switch and probably filter out message types you don't care about. 

For this, try a `Demux`, like `SwitchDemux`, which receives messages and type switches them to call functions with typed messages.

For example, say you're only interested in Tweets and Direct Messages.

```go
demux := twitter.NewSwitchDemux()
demux.Tweet = func(tweet *twitter.Tweet) {
    fmt.Println(tweet.Text)
}
demux.DM = func(dm *twitter.DirectMessage) {
    fmt.Println(dm.SenderID)
}
```

Pass the `Demux` each message or give it the entire `Stream.Message` channel.

```go
for message := range stream.Messages {
    demux.Handle(message)
}
// or pass the channel
demux.HandleChan(stream.Messages)
```

### Stopping

The `Stream` will stop itself if the stream disconnects and retrying produces unrecoverable errors. When this occurs, `Stream` will close the `stream.Messages` channel, so execution will break out of any message *for range* loops.

When you are finished receiving from a `Stream`, call `Stop()` which closes the connection, channels, and stops the goroutine **before** returning. This ensures resources are properly cleaned up.

### Pitfalls

**Bad**: In this example, `Stop()` is unlikely to be reached. Control stays in the message loop unless the `Stream` becomes disconnected and cannot retry.

```go
// program does not terminate :(
stream, _ := client.Streams.Sample(params)
for message := range stream.Messages {
    demux.Handle(message)
}
stream.Stop()
```

**Bad**: Here, messages are received on a non-main goroutine, but then `Stop()` is called immediately. The `Stream` is stopped and the program exits.

```go
// got no messages :(
stream, _ := client.Streams.Sample(params)
go demux.HandleChan(stream.Messages)
stream.Stop()
```

**Good**: For main package scripts, one option is to receive messages in a goroutine and wait for CTRL-C to be pressed, then explicitly stop the `Stream`.

```go
stream, err := client.Streams.Sample(params)
go demux.HandleChan(stream.Messages)

// Wait for SIGINT and SIGTERM (HIT CTRL-C)
ch := make(chan os.Signal)
signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
log.Println(<-ch)

stream.Stop()
```

## Authentication

The API client accepts an any `http.Client` capable of making user auth (OAuth1) or application auth (OAuth2) authorized requests. See the [dghubble/oauth1](https://github.com/dghubble/oauth1) and [golang/oauth2](https://github.com/golang/oauth2/) packages which can provide such agnostic clients.

Passing an `http.Client` directly grants you control over the underlying transport, avoids dependencies on particular OAuth1 or OAuth2 packages, and keeps client APIs separate from authentication protocols.

See the [google/go-github](https://github.com/google/go-github) client which takes the same approach.

For example, make requests as a consumer application on behalf of a user who has granted access, with OAuth1.

```go
// OAuth1
import (
    "github.com/dghubble/go-twitter/twitter"
    "github.com/dghubble/oauth1"
)

config := oauth1.NewConfig("consumerKey", "consumerSecret")
token := oauth1.NewToken("accessToken", "accessSecret")
// http.Client will automatically authorize Requests
httpClient := config.Client(oauth1.NoContext, token)

// Twitter client
client := twitter.NewClient(httpClient)
```

If no user auth context is needed, make requests as your application with application auth.

```go
// OAuth2
import (
    "github.com/dghubble/go-twitter/twitter"
    "golang.org/x/oauth2"
)

config := &oauth2.Config{}
token := &oauth2.Token{AccessToken: accessToken}
// http.Client will automatically authorize Requests
httpClient := config.Client(oauth2.NoContext, token)

// Twitter client
client := twitter.NewClient(httpClient)
```

To implement Login with Twitter for web or mobile, see the gologin [package](https://github.com/dghubble/gologin) and [examples](https://github.com/dghubble/gologin/tree/master/examples/twitter).

## Roadmap

* Support gzipped streams
* Auto-stop streams in the event of long stalls

## Contributing

See the [Contributing Guide](https://gist.github.com/dghubble/be682c123727f70bcfe7).

## License

[MIT License](LICENSE)
