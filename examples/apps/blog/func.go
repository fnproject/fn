package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Person struct {
	Name string
}

func main() {
	p := &Person{Name: "World"}
	json.NewDecoder(os.Stdin).Decode(p)
	mapD := map[string]string{
		"message": fmt.Sprintf("Hello %s", p.Name),
		"posts":   "http://localhost:8080/r/blog/posts",
	}
	mapB, _ := json.Marshal(mapD)
	fmt.Println(string(mapB))
}
