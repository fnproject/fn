package docker

import (
	"testing"
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
