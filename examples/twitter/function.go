// Copyright 2016 Iron.io
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	config := oauth1.NewConfig(os.Getenv("CONFIG_CUSTOMER_KEY"), os.Getenv("CONFIG_CUSTOMER_SECRET"))
	token := oauth1.NewToken(os.Getenv("CONFIG_ACCESS_TOKEN"), os.Getenv("CONFIG_ACCESS_SECRET"))

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
