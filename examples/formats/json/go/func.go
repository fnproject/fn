package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
)

type Person struct {
	Name string `json:"name"`
}

type JSONInput struct {
	Body string `json:"body"`
}

type JSONOutput struct {
	StatusCode int    `json:"status"`
	Body       string `json:"body"`
}

func main() {

	enc := json.NewEncoder(os.Stdout)
	r := bufio.NewReader(os.Stdin)
	for {
		var buf bytes.Buffer
		in := &JSONInput{}
		_, err := io.Copy(&buf, r)
		if err != nil {
			log.Fatalln(err)
		}

		err = json.Unmarshal(buf.Bytes(), in)
		if err != nil {
			log.Fatalln(err)
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
