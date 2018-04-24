package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

type Person struct {
	Name string
}

type JSON struct {
	Body string `json:"body,omitempty"`
}

func main() {
	in := &JSON{}
	err := json.NewDecoder(os.Stdin).Decode(in)
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
	}
	person := Person{}

	if len(in.Body) != 0 {
		if err := json.NewDecoder(bytes.NewReader([]byte(in.Body))).Decode(&person); err != nil {
			fmt.Fprintf(os.Stderr, err.Error())
			return
		}
	}
	if person.Name == "" {
		person.Name = "World"
	}

	res := &JSON{
		Body: fmt.Sprintf("Hello %s!", person.Name),
	}
	json.NewEncoder(os.Stdout).Encode(res)
}
