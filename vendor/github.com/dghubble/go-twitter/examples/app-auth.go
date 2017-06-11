package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/coreos/pkg/flagutil"
	"github.com/dghubble/go-twitter/twitter"
	"golang.org/x/oauth2"
)

func main() {
	flags := flag.NewFlagSet("app-auth", flag.ExitOnError)
	accessToken := flags.String("app-access-token", "", "Twitter Application Access Token")
	flags.Parse(os.Args[1:])
	flagutil.SetFlagsFromEnv(flags, "TWITTER")

	if *accessToken == "" {
		log.Fatal("Application Access Token required")
	}

	config := &oauth2.Config{}
	token := &oauth2.Token{AccessToken: *accessToken}
	// OAuth2 http.Client will automatically authorize Requests
	httpClient := config.Client(oauth2.NoContext, token)

	// Twitter client
	client := twitter.NewClient(httpClient)

	// user show
	userShowParams := &twitter.UserShowParams{ScreenName: "golang"}
	user, _, _ := client.Users.Show(userShowParams)
	fmt.Printf("USERS SHOW:\n%+v\n", user)

	// users lookup
	userLookupParams := &twitter.UserLookupParams{ScreenName: []string{"golang", "gophercon"}}
	users, _, _ := client.Users.Lookup(userLookupParams)
	fmt.Printf("USERS LOOKUP:\n%+v\n", users)

	// status show
	statusShowParams := &twitter.StatusShowParams{}
	tweet, _, _ := client.Statuses.Show(584077528026849280, statusShowParams)
	fmt.Printf("STATUSES SHOW:\n%+v\n", tweet)

	// statuses lookup
	statusLookupParams := &twitter.StatusLookupParams{ID: []int64{20}, TweetMode: "extended"}
	tweets, _, _ := client.Statuses.Lookup([]int64{573893817000140800}, statusLookupParams)
	fmt.Printf("STATUSES LOOKUP:\n%+v\n", tweets)

	// oEmbed status
	statusOembedParams := &twitter.StatusOEmbedParams{ID: 691076766878691329, MaxWidth: 500}
	oembed, _, _ := client.Statuses.OEmbed(statusOembedParams)
	fmt.Printf("OEMBED TWEET:\n%+v\n", oembed)

	// user timeline
	userTimelineParams := &twitter.UserTimelineParams{ScreenName: "golang", Count: 2}
	tweets, _, _ = client.Timelines.UserTimeline(userTimelineParams)
	fmt.Printf("USER TIMELINE:\n%+v\n", tweets)

	// search tweets
	searchTweetParams := &twitter.SearchTweetParams{
		Query:     "happy birthday",
		TweetMode: "extended",
		Count:     3,
	}
	search, _, _ := client.Search.Tweets(searchTweetParams)
	fmt.Printf("SEARCH TWEETS:\n%+v\n", search)
	fmt.Printf("SEARCH METADATA:\n%+v\n", search.Metadata)
}
