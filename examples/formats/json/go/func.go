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
	RequestURL string `json:"request_url"`
	CallID     string `json:"call_id"`
	Method     string `json:"method"`
	Body       string `json:"body"`
}

func (a *JSONInput) String() string {
	return fmt.Sprintf("request_url=%s\ncall_id=%s\nmethod=%s\n\nbody=%s",
		a.RequestURL, a.CallID, a.Method, a.Body)
}

type JSONOutput struct {
	StatusCode int `json:"status"`
	Body string `json:"body"`
}

func main() {
	// p := &Person{Name: "World"}
	// json.Unmarshal(os.Stdin).Decode(p)
	// mapD := map[string]string{"message": fmt.Sprintf("Hello %s", p.Name)}
	// mapB, _ := json.Marshal(mapD)
	// fmt.Println(string(mapB))

	dec := json.NewDecoder(os.Stdin)
	enc := json.NewEncoder(os.Stdout)
	var loopCounter = 0
	for {
		loopCounter++
		log.Println("loopCounter:", loopCounter)

		in := &JSONInput{}
		if err := dec.Decode(in); err != nil {
			log.Fatalln(err)
			return
		}
		log.Println("JSONInput: ", in)

		person := Person{}
                if in.Body != "" {
			if err := json.Unmarshal([]byte(in.Body), &person); err != nil {
				log.Fatalln(err)
			}
                }

		log.Println("Person: ", person)

		mapResult := map[string]string{"message": fmt.Sprintf("Hello %s", person.Name)}
		out := &JSONOutput{StatusCode: 200}
		b, _ := json.Marshal(mapResult)
		out.Body = string(b)
		if err := enc.Encode(out); err != nil {
			log.Fatalln(err)
		}
	}
}
