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
	mapD := map[string]string{"message": fmt.Sprintf("Hello %s", p.Name)}
	mapB, _ := json.Marshal(mapD)
	fmt.Println(string(mapB))

	// TODO: move these lines to a test, this was for testing log output issues
	log.Println("---> stderr goes to the server logs.")
	log.Println("---> LINE 2")
	log.Println("---> LINE 3 with a break right here\nand LINE 4")
	log.Println("---> LINE 5 with a double line break\n")
	log.Println("---> LINE 6")
}
