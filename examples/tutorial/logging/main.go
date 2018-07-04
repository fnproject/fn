package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
)

const lBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

type outputSize struct {
	Size int `json:"size"`
}

func randStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = lBytes[rand.Intn(len(lBytes))]
	}
	return string(b)
}

func main() {
	out := &outputSize{Size: 64 * 1024}
	json.NewDecoder(os.Stdin).Decode(out)
	fmt.Fprintln(os.Stderr, randStringBytes(out.Size))
}
