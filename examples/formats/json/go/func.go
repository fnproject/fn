package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

type Person struct {
	Name string `json:"name"`
}

type JSON struct {
	Headers    http.Header `json:"headers"`
	Body       string      `json:"body,omitempty"`
	StatusCode int         `json:"status,omitempty"`
}

func main() {

	stdin := json.NewDecoder(os.Stdin)
	stdout := json.NewEncoder(os.Stdout)
	stderr := json.NewEncoder(os.Stderr)
	for {
		in := &JSON{}

		err := stdin.Decode(in)
		if err != nil {
			log.Fatalf("Unable to decode incoming data: %s", err.Error())
			fmt.Fprintf(os.Stderr, err.Error())
		}
		person := Person{}
		stderr.Encode(in.Body)
		stderr.Encode(fmt.Sprintf(in.Body))
		if len(in.Body) != 0 {
			if err := json.NewDecoder(bytes.NewReader([]byte(in.Body))).Decode(&person); err != nil {
				log.Fatalf("Unable to decode Person object data: %s", err.Error())
				fmt.Fprintf(os.Stderr, err.Error())
			}
		}
		if person.Name == "" {
			person.Name = "World"
		}

		mapResult := map[string]string{"message": fmt.Sprintf("Hello %s", person.Name)}
		b, err := json.Marshal(mapResult)
		if err != nil {
			log.Fatalf("Unable to marshal JSON response body: %s", err.Error())
			fmt.Fprintf(os.Stderr, err.Error())
		}
		out := &JSON{
			StatusCode: http.StatusOK,
			Body:       string(b),
		}
		stderr.Encode(out)
		if err := stdout.Encode(out); err != nil {
			log.Fatalf("Unable to encode JSON response: %s", err.Error())
			fmt.Fprintf(os.Stderr, err.Error())
		}
	}
}
