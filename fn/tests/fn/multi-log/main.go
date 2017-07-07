package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	fmt.Fprintln(os.Stderr, "First line")
	fmt.Fprintln(os.Stdout, "Ok")
	time.Sleep(3 * time.Second)
	fmt.Fprintln(os.Stderr, "Second line")
}
