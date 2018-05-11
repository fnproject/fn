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
	mapD := map[string]string{"message": fmt.Sprintf("Hello %s!", p.Name)}
	mapB, _ := json.Marshal(mapD)
	fmt.Println(string(mapB))
}
