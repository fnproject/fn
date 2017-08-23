package client

import (
	"os"

	"log"
	"net/url"

	fnclient "github.com/funcy/functions_go/client"
	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
)

const (
	envFnToken = "FN_TOKEN"
)

func Host() string {
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

func APIClient() *fnclient.Functions {
	transport := httptransport.New(Host(), "/v1", []string{"http"})
	if os.Getenv(envFnToken) != "" {
		transport.DefaultAuthentication = httptransport.BearerToken(os.Getenv(envFnToken))
	}

	// create the API client, with the transport
	client := fnclient.New(transport, strfmt.Default)

	return client
}
