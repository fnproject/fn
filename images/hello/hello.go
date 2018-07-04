package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type person struct {
	Name string
}

func main() {
	n := os.Getenv("NAME") // can grab name from env or input
	if n == "" {
		n = "World"
	}
	p := &person{Name: n}
	json.NewDecoder(os.Stdin).Decode(p)
	fmt.Printf("Hello %v!\n", p.Name)
}
