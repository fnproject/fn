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
	p := &person{Name: "World"}
	json.NewDecoder(os.Stdin).Decode(p)
	mapD := map[string]string{"message": fmt.Sprintf("Hello %s", p.Name)}
	mapD["SECRET_1"] = os.Getenv("SECRET_1")
	mapD["SECRET_2"] = os.Getenv("SECRET_2")
	mapB, _ := json.Marshal(mapD)
	fmt.Println(string(mapB))
}
