package main

import (
	"fmt"
	"os"
)

func main() {
	envvar := os.Getenv("HEADER_ENVVAR")
	if envvar != "" {
		fmt.Println("HEADER_ENVVAR:", envvar)
	}
	fmt.Println("hw")
}
