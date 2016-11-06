package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Input struct {
	Name string `json:"name"`
}

func main() {
	input := &Input{}
	if err := json.NewDecoder(os.Stdin).Decode(input); err != nil {
		// log.Println("Bad payload or no payload. Ignoring.", err)
	}
	if input.Name == "" {
		input.Name = "World"
	}
	fmt.Printf("Hello %v!\n", input.Name)
}
