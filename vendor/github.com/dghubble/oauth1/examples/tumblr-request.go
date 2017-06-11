package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/dghubble/oauth1"
)

// Tumblr access token (token credential) requests on behalf of a user
func main() {
	// read credentials from environment variables
	consumerKey := os.Getenv("TUMBLR_CONSUMER_KEY")
	consumerSecret := os.Getenv("TUMBLR_CONSUMER_SECRET")
	accessToken := os.Getenv("TUMBLR_ACCESS_TOKEN")
	accessSecret := os.Getenv("TUMBLR_ACCESS_SECRET")
	if consumerKey == "" || consumerSecret == "" || accessToken == "" || accessSecret == "" {
		panic("Missing required environment variable")
	}

	config := oauth1.NewConfig(consumerKey, consumerSecret)
	token := oauth1.NewToken(accessToken, accessSecret)

	// httpClient will automatically authorize http.Request's
	httpClient := config.Client(oauth1.NoContext, token)

	// get information about the current authenticated user
	path := "https://api.tumblr.com/v2/user/info"
	resp, _ := httpClient.Get(path)
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Printf("Raw Response Body:\n%v\n", string(body))

	// note: Tumblr requires OAuth signed requests for particular endpoints,
	// others just need a consumer key query parameter (its janky).
}
