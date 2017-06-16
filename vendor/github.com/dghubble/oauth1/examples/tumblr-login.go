package main

import (
	"fmt"
	"log"
	"os"

	"github.com/dghubble/oauth1"
	"github.com/dghubble/oauth1/tumblr"
)

var config oauth1.Config

// main performs the Tumblr OAuth1 user flow from the command line
func main() {
	// read credentials from environment variables
	consumerKey := os.Getenv("TUMBLR_CONSUMER_KEY")
	consumerSecret := os.Getenv("TUMBLR_CONSUMER_SECRET")
	if consumerKey == "" || consumerSecret == "" {
		log.Fatal("Required environment variable missing.")
	}

	config = oauth1.Config{
		ConsumerKey:    consumerKey,
		ConsumerSecret: consumerSecret,
		// Tumblr does not support oob, uses consumer registered callback
		CallbackURL: "",
		Endpoint:    tumblr.Endpoint,
	}

	requestToken, requestSecret, err := login()
	if err != nil {
		log.Fatalf("Request Token Phase: %s", err.Error())
	}
	accessToken, err := receivePIN(requestToken, requestSecret)
	if err != nil {
		log.Fatalf("Access Token Phase: %s", err.Error())
	}

	fmt.Println("Consumer was granted an access token to act on behalf of a user.")
	fmt.Printf("token: %s\nsecret: %s\n", accessToken.Token, accessToken.TokenSecret)
}

func login() (requestToken, requestSecret string, err error) {
	requestToken, requestSecret, err = config.RequestToken()
	if err != nil {
		return "", "", err
	}
	authorizationURL, err := config.AuthorizationURL(requestToken)
	if err != nil {
		return "", "", err
	}
	fmt.Printf("Open this URL in your browser:\n%s\n", authorizationURL.String())
	return requestToken, requestSecret, err
}

func receivePIN(requestToken, requestSecret string) (*oauth1.Token, error) {
	fmt.Printf("Choose whether to grant the application access.\nPaste " +
		"the oauth_verifier parameter (excluding trailing #_=_) from the " +
		"address bar: ")
	var verifier string
	_, err := fmt.Scanf("%s", &verifier)
	accessToken, accessSecret, err := config.AccessToken(requestToken, requestSecret, verifier)
	if err != nil {
		return nil, err
	}
	return oauth1.NewToken(accessToken, accessSecret), err
}
