package drivers

import (
	"testing"
)

func TestParseImage(t *testing.T) {
	cases := map[string][]string{
		"fnproject/fn-test-utils":                           {"", "fnproject/fn-test-utils", "latest"},
		"fnproject/fn-test-utils:v1":                        {"", "fnproject/fn-test-utils", "v1"},
		"my.registry/fn-test-utils":                         {"my.registry", "fn-test-utils", "latest"},
		"my.registry/fn-test-utils:v1":                      {"my.registry", "fn-test-utils", "v1"},
		"mongo":                                             {"", "library/mongo", "latest"},
		"mongo:v1":                                          {"", "library/mongo", "v1"},
		"quay.com/fnproject/fn-test-utils":                  {"quay.com", "fnproject/fn-test-utils", "latest"},
		"quay.com:8080/fnproject/fn-test-utils:v2":          {"quay.com:8080", "fnproject/fn-test-utils", "v2"},
		"localhost.localdomain:5000/samalba/hipache:latest": {"localhost.localdomain:5000", "samalba/hipache", "latest"},
		"localhost.localdomain:5000/samalba/hipache/isthisallowedeven:latest": {"localhost.localdomain:5000", "samalba/hipache/isthisallowedeven", "latest"},
	}

	for in, out := range cases {
		reg, repo, tag := ParseImage(in)
		if reg != out[0] || repo != out[1] || tag != out[2] {
			t.Errorf("Test input %q wasn't parsed as expected. Expected %q, got %q", in, out, []string{reg, repo, tag})
		}
	}
}
