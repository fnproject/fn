package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
)

type payload struct {
	Username string `json:"username"`
}

func main() {
	username := "getiron"

	// Getting username in payload
	envPayload := os.Getenv("PAYLOAD")
	if envPayload != "" {
		var pl payload

		err := json.Unmarshal([]byte(envPayload), &pl)
		if err != nil {
			log.Println("Invalid payload")
			return
		}

		if pl.Username != "" {
			username = pl.Username
		}
	}

	fmt.Println("Looking for tweets of the account:", username)

	// Twitter auth config
	config := oauth1.NewConfig(os.Getenv("CUSTOMER_KEY"), os.Getenv("CUSTOMER_SECRET"))
	token := oauth1.NewToken(os.Getenv("ACCESS_TOKEN"), os.Getenv("ACCESS_SECRET"))

	httpClient := config.Client(oauth1.NoContext, token)

	// Twitter client
	client := twitter.NewClient(httpClient)

	// Load tweets
	tweets, _, err := client.Timelines.UserTimeline(&twitter.UserTimelineParams{ScreenName: username})
	if err != nil {
		fmt.Println("Error loading tweets: ", err)
	}

	// Show tweets
	for _, tweet := range tweets {
		fmt.Println(tweet.User.Name + ": " + tweet.Text)
	}

}
