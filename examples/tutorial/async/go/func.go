package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

type person struct {
	Name string
}

func main() {
	p := &person{Name: "World"}
	json.NewDecoder(os.Stdin).Decode(p)
	fmt.Printf("Hello %v!\n", p.Name)

	log.Println("---> stderr goes to the server logs.")
}
