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

func main() {
	p := &Person{Name: "World"}
	json.NewDecoder(os.Stdin).Decode(p)
	fmt.Printf("Hello %v!\n", p.Name)

	log.Println("---> stderr goes to the server logs.")
	log.Println("---> LINE 2")
	log.Println("---> LINE 3 with a break right here\nand LINE 4")
	log.Println("---> LINE 5 with a double line break\n")
	log.Println("---> LINE 6")
}
