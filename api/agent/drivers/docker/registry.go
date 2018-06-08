package docker

import (
	"net/url"
	"strings"
)

const hubURL = "https://registry.hub.docker.com"

func registryURL(addr string) (string, error) {
	if addr == "" || strings.Contains(addr, "hub.docker.com") || strings.Contains(addr, "index.docker.io") {
		return hubURL, nil
	}

	uri, err := url.Parse(addr)
	if err != nil {
		return "", err
	}

	if uri.Scheme == "" {
		uri.Scheme = "https"
	}
	uri.Path = strings.TrimSuffix(uri.Path, "/")
	uri.Path = strings.TrimPrefix(uri.Path, "/v2")
	uri.Path = strings.TrimPrefix(uri.Path, "/v1") // just try this, if it fails it fails, not supporting v1
	return uri.String(), nil
}
