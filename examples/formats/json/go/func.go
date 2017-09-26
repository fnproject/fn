package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

type Person struct {
	Name string
}

type JSONInput struct {
	Body string `json:"body"`
}

type JSONOutput struct {
	StatusCode int    `json:"status"`
	Body       string `json:"body"`
}

func main() {

	dec := json.NewDecoder(os.Stdin)
	enc := json.NewEncoder(os.Stdout)
	for {

		in := &JSONInput{}
		if err := dec.Decode(in); err != nil {
			log.Fatalln(err)
			return
		}

		person := Person{}
		if in.Body != "" {
			if err := json.Unmarshal([]byte(in.Body), &person); err != nil {
				log.Fatalln(err)
			}
		}
		if person.Name == "" {
			person.Name = "World"
		}

		mapResult := map[string]string{"message": fmt.Sprintf("Hello %s", person.Name)}
		out := &JSONOutput{StatusCode: 200}
		b, _ := json.Marshal(mapResult)
		out.Body = string(b)
		if err := enc.Encode(out); err != nil {
			log.Fatalln(err)
		}
	}
}
