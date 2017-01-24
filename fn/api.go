package main

import (
	"os"

	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	fnclient "github.com/iron-io/functions_go/client"
	"log"
	"net/url"
)

func host() string {
	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8080"
	}

	u, err := url.Parse(apiURL)
	if err != nil {
		log.Fatalln("Couldn't parse API URL:", err)
	}

	return u.Host
}

func apiClient() *fnclient.Functions {
	transport := httptransport.New(host(), "/v1", []string{"http"})
	if os.Getenv("IRON_TOKEN") != "" {
		transport.DefaultAuthentication = httptransport.BearerToken(os.Getenv("IRON_TOKEN"))
	}

	// create the API client, with the transport
	client := fnclient.New(transport, strfmt.Default)

	return client
}
