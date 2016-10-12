package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

type Input struct {
	Name string `json:"name"`
}

func main() {
	for _, e := range os.Environ() {
		fmt.Println(e)
	}
	input := &Input{}
	if err := json.NewDecoder(os.Stdin).Decode(input); err != nil {
		log.Fatalln("Error! Bad input. ", err)
	}
	if input.Name == "" {
		input.Name = "World"
	}
	fmt.Printf("Hello %v!\n", input.Name)
}
