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
	mapD := map[string]string{"message": fmt.Sprintf("Hello %s", p.Name)}
	mapB, _ := json.Marshal(mapD)
	fmt.Println(string(mapB))

	log.Println("---> stderr goes to the server logs.")
}
