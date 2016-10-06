package main

import (
	"os"
	"fmt"
	"io/ioutil"
)

func main() {
	bytes, _ := ioutil.ReadAll(os.Stdin)
	fmt.Printf("Hello %v!\n", string(bytes))
}
