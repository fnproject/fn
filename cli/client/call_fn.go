package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const FN_CALL_ID = "Fn_call_id"

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

type apiErr struct {
	Message string `json:"message"`
}

type callID struct {
	CallID string `json:"call_id"`
	Error  apiErr `json:"error"`
}

func CallFN(u string, content io.Reader, output io.Writer, method string, env []string, includeCallID bool) error {
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
	// for sync calls
	if call_id, found := resp.Header[FN_CALL_ID]; found {
		if includeCallID {
			fmt.Fprint(os.Stderr, fmt.Sprintf("Call ID: %v\n", call_id[0]))
		}
		io.Copy(output, resp.Body)
	} else {
		// for async calls and error discovering
		c := &callID{}
		err = json.NewDecoder(resp.Body).Decode(c)
		if err == nil {
			// decode would not fail in both cases:
			// - call id in body
			// - error in body
			// that's why we need to check values of attributes
			if c.CallID != "" {
				fmt.Fprint(os.Stderr, fmt.Sprintf("Call ID: %v\n", c.CallID))
			} else {
				fmt.Fprint(output, fmt.Sprintf("Error: %v\n", c.Error.Message))
			}
		} else {
			return err
		}
	}

	if resp.StatusCode >= 400 {
		// TODO: parse out error message
		return fmt.Errorf("error calling function: status %v", resp.StatusCode)
	}

	return nil
}
