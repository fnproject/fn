package main

import (
	"fmt"
	"os"
)

func main() {
	envvar := os.Getenv("FN_HEADER_ENVVAR")
	if envvar != "" {
		fmt.Println("FN_HEADER_ENVVAR:", envvar)
	}
	fmt.Println("hw")
}
