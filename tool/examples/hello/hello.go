package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Printf("Hello %v!\n", os.Getenv("PAYLOAD"))
}
