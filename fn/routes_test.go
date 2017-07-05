package main

import (
	"net/http"
	"os"
	"testing"
	"gitlab-odx.oracle.com/odx/functions/fn/utils"
)

func TestEnvAsHeader(t *testing.T) {
	const expectedValue = "v=v"
	os.Setenv("k", expectedValue)

	cases := [][]string{
		nil,
		[]string{},
		[]string{"k"},
	}
	for _, selectedEnv := range cases {
		req, _ := http.NewRequest("GET", "http://www.example.com", nil)
		utils.EnvAsHeader(req, selectedEnv)
		if found := req.Header.Get("k"); found != expectedValue {
			t.Errorf("not found expected header: %v", found)
		}
	}
}
