package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

func EnvAsHeader(req *http.Request, selectedEnv []string) {
	detectedEnv := os.Environ()
	if len(selectedEnv) > 0 {
		detectedEnv = selectedEnv
	}

	for _, e := range detectedEnv {
		kv := strings.Split(e, "=")
		name := kv[0]
		req.Header.Set(name, os.Getenv(name))
	}
}

type callID struct {
	CallID string `json:"call_id"`
}

func CallFN(u string, content io.Reader, output io.Writer, method string, env []string) error {
	if method == "" {
		if content == nil {
			method = "GET"
		} else {
			method = "POST"
		}
	}

	req, err := http.NewRequest(method, u, content)
	if err != nil {
		return fmt.Errorf("error running route: %s", err)
	}

	req.Header.Set("Content-Type", "application/json")

	if len(env) > 0 {
		EnvAsHeader(req, env)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error running route: %s", err)
	}
	if call_id, found := resp.Header["Fn_call_id"]; found {
		fmt.Fprint(output, fmt.Sprintf("Call ID: %v\n", call_id[0]))
		io.Copy(output, resp.Body)
	} else {
		c := &callID{}
		json.NewDecoder(resp.Body).Decode(c)
		fmt.Fprint(output, fmt.Sprintf("Call ID: %v\n", c.CallID))
	}

	if resp.StatusCode >= 400 {
		// TODO: parse out error message
		return fmt.Errorf("error calling function: status %v", resp.StatusCode)
	}

	return nil
}
