package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type status struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
}

func main() {

	var s status
	err := json.NewDecoder(os.Stdin).Decode(&s)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	if s.Status != true {
		fmt.Println(s.Message)
		os.Exit(1)
	}

	fmt.Println("OK")
}
