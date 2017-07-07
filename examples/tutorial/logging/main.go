package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
)

const lBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

type OutputSize struct {
	Size int `json:"size"`
}

func RandStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = lBytes[rand.Intn(len(lBytes))]
	}
	return string(b)
}

func main() {
	out := &OutputSize{Size: 64 * 1024}
	json.NewDecoder(os.Stdin).Decode(out)
	fmt.Fprintln(os.Stderr, RandStringBytes(out.Size))
}
