package docker

import (
	"testing"

	"honnef.co/go/tools/config"
)

func verify(expected []string, checks map[string]bool) bool {
	if len(expected) != len(checks) {
		return false
	}
	for _, v := range expected {
		_, ok := checks[v]
		if !ok {
			return false
		}
	}
	return true
}

func TestRegistrySubDomains(t *testing.T) {
	var exp []string
	var res map[string]bool

	exp = []string{"google.com"}
	res = getSubdomains("google.com")
	if !verify(exp, res) {
		t.Fatalf("subdomain results failed expected[%+v] != results[%+v]", exp, res)
	}

	exp = []string{"top.google.com", "google.com"}
	res = getSubdomains("top.google.com")
	if !verify(exp, res) {
		t.Fatalf("subdomain results failed expected[%+v] != results[%+v]", exp, res)
	}

	exp = []string{"top.google.com:443", "google.com:443"}
	res = getSubdomains("top.google.com:443")
	if !verify(exp, res) {
		t.Fatalf("subdomain results failed expected[%+v] != results[%+v]", exp, res)
	}

	exp = []string{"top.top.google.com", "top.google.com", "google.com"}
	res = getSubdomains("top.top.google.com")
	if !verify(exp, res) {
		t.Fatalf("subdomain results failed expected[%+v] != results[%+v]", exp, res)
	}

	exp = []string{"docker"}
	res = getSubdomains("docker")
	if !verify(exp, res) {
		t.Fatalf("subdomain results failed expected[%+v] != results[%+v]", exp, res)
	}

	exp = []string{""}
	res = getSubdomains("")
	if !verify(exp, res) {
		t.Fatalf("subdomain results failed expected[%+v] != results[%+v]", exp, res)
	}
}

func TestRegistryEnv(t *testing.T) {

	testCfg := `{
	"auths":{
		"https://my.registry.com":{"auth":"Y29jbzpjaGVlc2UK"},
		"https://my.registry.com:5000":{"auth":"Y29jbzpjaGVlc2UK"},
		"https://index.docker.io/v2/":{"auth":"Y29jbzpjaGVlc2UK"},
		"rawregistry.com":{"auth":"Y29jbzpjaGVlc2UK"}
	}}`

	cfg, err := config.Load("") // docker initializes from home/env var
	if err != nil {
		t.Fatalf("parsing test cfg failed: %s", err)
	}

	drvAuths, err := preprocessAuths(cfg.AuthConfigs)
	if err != nil {
		t.Fatalf("preprocess test cfg failed: %s", err)
	}

	res := findRegistryConfig("", drvAuths)
	if res == nil || res.ServerAddress != "https://index.docker.io/v2/" {
		t.Fatalf("empty registry should pickup docker %v", res)
	}

	res = findRegistryConfig("docker.io", drvAuths)
	if res == nil || res.ServerAddress != "https://index.docker.io/v2/" {
		t.Fatalf("docker.io registry should pickup docker %v", res)
	}

	res = findRegistryConfig("localhost", drvAuths)
	if res == nil || res.ServerAddress != "" {
		t.Fatalf("localhost registry should pickup a default (empty) cfg %v", res)
	}

	res = findRegistryConfig("registry.com", drvAuths)
	if res == nil || res.ServerAddress != "https://my.registry.com" {
		t.Fatalf("registry.com registry should pickup my.registry.com cfg %v", res)
	}

	res = findRegistryConfig("my.registry.com", drvAuths)
	if res == nil || res.ServerAddress != "https://my.registry.com" {
		t.Fatalf("my.registry.com registry should pickup my.registry.com cfg %v", res)
	}

	res = findRegistryConfig("registry.com:5000", drvAuths)
	if res == nil || res.ServerAddress != "https://my.registry.com:5000" {
		t.Fatalf("registry.com:5000 registry should pickup my.registry.com:5000 cfg %v", res)
	}
	res = findRegistryConfig("rawregistry.com", drvAuths)
	if res == nil || res.ServerAddress != "rawregistry.com" {
		t.Fatalf("rawregistry.com registry should pickup rawregistry.com cfg %v", res)
	}
}
